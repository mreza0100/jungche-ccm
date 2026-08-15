package headless

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// conversation is a chat whose transcript a test writes by hand, plus the
// resolver Await calls to find it. The chat's state is written from the test's
// own goroutine while Await reads it from another, so it is mutex-guarded:
// this fixture models a live seat, and a live seat changes under its watcher.
type conversation struct {
	path    string
	guard   sync.Mutex
	chat    Chat
	found   bool
	resolve func(context.Context) (Chat, bool, error)
}

func (talk *conversation) vanish() {
	talk.guard.Lock()
	defer talk.guard.Unlock()
	talk.found = false
}

func (talk *conversation) appear() {
	talk.guard.Lock()
	defer talk.guard.Unlock()
	talk.found = true
}

func (talk *conversation) die() {
	talk.guard.Lock()
	defer talk.guard.Unlock()
	talk.chat.Live = false
}

func newConversation(t *testing.T) *conversation {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chat.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	talk := &conversation{
		path:  path,
		found: true,
		chat: Chat{
			Name:   "worker",
			Engine: "cc",
			Path:   path,
			Socket: "cc-1-2-3",
			Live:   true,
		},
	}
	talk.resolve = func(context.Context) (Chat, bool, error) {
		talk.guard.Lock()
		defer talk.guard.Unlock()
		return talk.chat, talk.found, nil
	}
	return talk
}

// say appends to the transcript. It panics rather than calling t.Fatal because
// tests call it from their own goroutines, where a Fatal would abandon the
// test instead of failing it.
func (talk *conversation) say(lines ...string) {
	file, err := os.OpenFile(talk.path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	for _, line := range lines {
		if _, err := file.WriteString(line + "\n"); err != nil {
			panic(err)
		}
	}
}

func user(text string) string {
	return `{"type":"user","message":{"content":"` + text + `"}}`
}

func assistant(text string) string {
	return `{"type":"assistant","message":{"content":[{"type":"text","text":"` +
		text + `"}]}}`
}

func tool(name string) string {
	return `{"type":"assistant","message":{"content":[{"type":"tool_use",` +
		`"name":"` + name + `","input":{}}]}}`
}

func fastOptions() AwaitOptions {
	return AwaitOptions{
		Poll:         5 * time.Millisecond,
		Settle:       60 * time.Millisecond,
		ResolveEvery: 5 * time.Millisecond,
		Timeout:      10 * time.Second,
	}
}

// TestAwaitReturnsTheAnswerToTheQuestionJustAsked is the two-way contract: the
// frontier is taken before the message, so nothing said earlier can be
// mistaken for the reply.
func TestAwaitReturnsTheAnswerToTheQuestionJustAsked(t *testing.T) {
	talk := newConversation(t)
	talk.say(user("an older question"), assistant("an older answer"))
	frontier, err := Frontier(talk.chat)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		talk.say(user("the new question"))
		time.Sleep(20 * time.Millisecond)
		talk.say(assistant("the new answer"))
	}()

	options := fastOptions()
	options.Offset = frontier
	turn, err := Await(context.Background(), talk.resolve, options)
	if err != nil {
		t.Fatalf("Await() error = %v", err)
	}
	if !turn.Delivered {
		t.Fatal("Delivered = false, want the human turn to count as proof")
	}
	if turn.Answer != "the new answer" {
		t.Fatalf("Answer = %q, want only the answer written after the question", turn.Answer)
	}
	if turn.State != StateIdle {
		t.Fatalf("State = %q, want %q", turn.State, StateIdle)
	}
}

// TestAwaitWaitsThroughToolWork is the difference between a model pausing to
// think and a model that has finished: an assistant line followed by tool work
// is a preamble, not an answer.
func TestAwaitWaitsThroughToolWork(t *testing.T) {
	talk := newConversation(t)
	go func() {
		talk.say(user("do the thing"), assistant("let me look"))
		time.Sleep(30 * time.Millisecond)
		talk.say(tool("Bash"))
		time.Sleep(30 * time.Millisecond)
		talk.say(assistant("done, it was the firewall"))
	}()

	turn, err := Await(context.Background(), talk.resolve, fastOptions())
	if err != nil {
		t.Fatalf("Await() error = %v", err)
	}
	if !strings.Contains(turn.Answer, "done, it was the firewall") {
		t.Fatalf("Answer = %q, want the finished answer", turn.Answer)
	}
	if turn.Tools != 1 {
		t.Fatalf("Tools = %d, want 1", turn.Tools)
	}
}

// TestAwaitTimesOutWithDeliveryIntact separates "you were not heard" from "you
// were heard and the answer is not in yet" — different facts with different
// recoveries.
func TestAwaitTimesOutWithDeliveryIntact(t *testing.T) {
	talk := newConversation(t)
	talk.say(user("a question nobody answers"))

	options := fastOptions()
	options.Timeout = 80 * time.Millisecond
	turn, err := Await(context.Background(), talk.resolve, options)
	if !errors.Is(err, ErrAwaitTimeout) {
		t.Fatalf("Await() error = %v, want ErrAwaitTimeout", err)
	}
	if !turn.Delivered {
		t.Fatal("Delivered = false, want the question to still count as delivered")
	}
	if turn.Answer != "" {
		t.Fatalf("Answer = %q, want none", turn.Answer)
	}
}

func TestAwaitReportsAChatThatDies(t *testing.T) {
	talk := newConversation(t)
	go func() {
		time.Sleep(20 * time.Millisecond)
		talk.vanish()
	}()
	_, err := Await(context.Background(), talk.resolve, fastOptions())
	if !errors.Is(err, ErrChatGone) {
		t.Fatalf("Await() error = %v, want ErrChatGone", err)
	}
}

// TestAwaitKeepsWhatADyingChatManagedToSay: an answer written before the seat
// went away is a real answer and must not be thrown out with the corpse.
func TestAwaitKeepsWhatADyingChatManagedToSay(t *testing.T) {
	talk := newConversation(t)
	go func() {
		time.Sleep(10 * time.Millisecond)
		talk.say(user("last words?"), assistant("goodbye"))
		time.Sleep(10 * time.Millisecond)
		talk.die()
	}()
	options := fastOptions()
	options.Settle = 5 * time.Second
	turn, err := Await(context.Background(), talk.resolve, options)
	if !errors.Is(err, ErrChatGone) {
		t.Fatalf("Await() error = %v, want ErrChatGone", err)
	}
	if turn.Answer != "goodbye" {
		t.Fatalf("Answer = %q, want what it said before it died", turn.Answer)
	}
	if turn.State != StateDead {
		t.Fatalf("State = %q, want %q", turn.State, StateDead)
	}
}

// TestAwaitWaitsOutTheGraceForAChatBeingBorn covers `run --await`: the seat is
// created, then indexed, and a lookup in between must not call it gone.
func TestAwaitWaitsOutTheGraceForAChatBeingBorn(t *testing.T) {
	talk := newConversation(t)
	talk.vanish()
	go func() {
		time.Sleep(30 * time.Millisecond)
		talk.say(user("hello"), assistant("hello back"))
		talk.appear()
	}()
	options := fastOptions()
	options.Grace = 5 * time.Second
	turn, err := Await(context.Background(), talk.resolve, options)
	if err != nil {
		t.Fatalf("Await() error = %v", err)
	}
	if turn.Answer != "hello back" {
		t.Fatalf("Answer = %q", turn.Answer)
	}
}

// TestAwaitStopsAtDeliveryWhenAskedTo is the launch proof: `run` needs to know
// the model was ASKED, and must not sit through the whole answer to learn it.
func TestAwaitStopsAtDeliveryWhenAskedTo(t *testing.T) {
	talk := newConversation(t)
	go func() {
		time.Sleep(10 * time.Millisecond)
		talk.say(user("the launch prompt"))
	}()
	options := fastOptions()
	options.StopOnDelivery = true
	options.Settle = time.Minute
	turn, err := Await(context.Background(), talk.resolve, options)
	if err != nil {
		t.Fatalf("Await() error = %v", err)
	}
	if !turn.Delivered || turn.Answer != "" {
		t.Fatalf("turn = %+v, want delivery alone", turn)
	}
}

// TestAwaitAnswersTheLatestQuestion: a second human turn arriving mid-wait
// makes everything said before it the answer to something else.
func TestAwaitAnswersTheLatestQuestion(t *testing.T) {
	talk := newConversation(t)
	go func() {
		talk.say(user("first"), assistant("answer to first"))
		time.Sleep(20 * time.Millisecond)
		talk.say(user("second"))
		time.Sleep(20 * time.Millisecond)
		talk.say(assistant("answer to second"))
	}()
	turn, err := Await(context.Background(), talk.resolve, fastOptions())
	if err != nil {
		t.Fatalf("Await() error = %v", err)
	}
	if turn.Answer != "answer to second" {
		t.Fatalf("Answer = %q, want only the answer to the last question", turn.Answer)
	}
}

func TestAwaitStreamsProgress(t *testing.T) {
	talk := newConversation(t)
	var progress bytes.Buffer
	go func() {
		talk.say(user("go"), tool("Read"), assistant("read it"))
	}()
	options := fastOptions()
	options.Progress = &progress
	if _, err := Await(context.Background(), talk.resolve, options); err != nil {
		t.Fatalf("Await() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(progress.String()), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "U ") ||
		!strings.HasPrefix(lines[1], "T ") || !strings.HasPrefix(lines[2], "A ") {
		t.Fatalf("progress = %q, want one condensed line per entry", progress.String())
	}
}

// TestAwaitFlagsAnAnswerSomebodyElseMayOwn: two callers can hold the same chat
// at once, and the second question makes the first caller's claim on the
// answer unprovable. Returning the newest answer is the useful behaviour; not
// SAYING so would be the dishonest one.
func TestAwaitFlagsAnAnswerSomebodyElseMayOwn(t *testing.T) {
	talk := newConversation(t)
	go func() {
		talk.say(user("my question"))
		time.Sleep(20 * time.Millisecond)
		talk.say(user("somebody else's question"))
		time.Sleep(20 * time.Millisecond)
		talk.say(assistant("one answer for both"))
	}()
	turn, err := Await(context.Background(), talk.resolve, fastOptions())
	if err != nil {
		t.Fatalf("Await() error = %v", err)
	}
	if !turn.Superseded {
		t.Fatal("Superseded = false after a second human turn landed mid-wait")
	}
	if turn.Answer != "one answer for both" {
		t.Fatalf("Answer = %q", turn.Answer)
	}
}
