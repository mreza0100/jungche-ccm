package chatgroup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGroupLifecycleAndDoorbellSuppression(t *testing.T) {
	bus, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := bus.Create(ctx, "lab", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := bus.Subscribe(ctx, "lab", "beta"); err != nil {
		t.Fatal(err)
	}
	var nudged []string
	nudge := func(_ context.Context, member, _ string) error {
		nudged = append(nudged, member)
		return nil
	}
	first, err := bus.Send(ctx, "lab", "alpha", "one", "", nudge)
	if err != nil {
		t.Fatal(err)
	}
	if first.Number != 1 || len(first.Nudges) != 1 || first.Nudges[0].Status != "nudged" {
		t.Fatalf("first send = %#v", first)
	}
	second, err := bus.Send(ctx, "lab", "alpha", "two", "", nudge)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Nudges) != 1 || second.Nudges[0].Status != "backlog" || len(nudged) != 1 {
		t.Fatalf("second send = %#v nudged=%v", second, nudged)
	}
	read, err := bus.Read(ctx, "lab", "beta", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Messages) != 2 || read.Cursor != 2 {
		t.Fatalf("read = %#v", read)
	}
	third, err := bus.Send(ctx, "lab", "alpha", "three", "bet*", nudge)
	if err != nil {
		t.Fatal(err)
	}
	if third.TargetMatches != 1 || len(nudged) != 2 {
		t.Fatalf("targeted send = %#v nudged=%v", third, nudged)
	}
	peek, err := bus.Read(ctx, "lab", "beta", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(peek.Messages) != 1 || !peek.Peek || peek.Cursor != 2 {
		t.Fatalf("peek = %#v", peek)
	}
}

func TestParallelSendsKeepOneRecordPerMessage(t *testing.T) {
	bus, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := bus.Create(ctx, "hammer", "sender"); err != nil {
		t.Fatal(err)
	}
	const count = 40
	var wait sync.WaitGroup
	errors := make(chan error, count)
	for index := 0; index < count; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, err := bus.Send(ctx, "hammer", "sender", fmt.Sprintf("message-%02d", index), "", nil)
			errors <- err
		}(index)
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	peek, err := bus.Read(ctx, "hammer", "", count+1)
	if err != nil {
		t.Fatal(err)
	}
	if len(peek.Messages) != count || peek.Total != count {
		t.Fatalf("parallel ledger count=%d total=%d, want %d", len(peek.Messages), peek.Total, count)
	}
}

func TestMalformedStateIsAnErrorNotAnEmptyListing(t *testing.T) {
	root := t.TempDir()
	bus, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "broken"), 0o700); err != nil {
		t.Fatal(err)
	}
	if groups, err := bus.List(context.Background(), "member"); err == nil || groups != nil {
		t.Fatalf("List = %#v, %v; want named malformed-group error", groups, err)
	}
}

func TestInviteUsesTransportWithoutSubscribingTarget(t *testing.T) {
	bus, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := bus.Create(ctx, "crew", "alpha"); err != nil {
		t.Fatal(err)
	}
	var target, body string
	receipt, err := bus.Invite(ctx, "crew", "alpha", "beta", func(_ context.Context, gotTarget, gotBody string) error {
		target, body = gotTarget, gotBody
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if target != "beta" || body == "" || receipt.Member != "beta" {
		t.Fatalf("invite=%#v target=%q body=%q", receipt, target, body)
	}
	groups, err := bus.List(ctx, "beta")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Member {
		t.Fatalf("invitation subscribed target: %#v", groups)
	}
}

func TestUnsubscribedCallerCannotSendOrAdvanceACursor(t *testing.T) {
	bus, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := bus.Create(ctx, "closed", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := bus.Send(ctx, "closed", "stranger", "intrusion", "", nil); err == nil {
		t.Fatal("unsubscribed sender appended to the ledger")
	}
	if _, err := bus.Read(ctx, "closed", "stranger", 0); err == nil {
		t.Fatal("unsubscribed reader advanced a cursor")
	}
}

func TestLongMessageSpillsCannotCollideAtTheSameTimestamp(t *testing.T) {
	bus, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bus.Now = func() time.Time { return time.Unix(1_700_000_000, 123) }
	ctx := context.Background()
	if _, err := bus.Create(ctx, "long", "alpha"); err != nil {
		t.Fatal(err)
	}
	first, err := bus.Send(ctx, "long", "alpha", strings.Repeat("a", 500), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := bus.Send(ctx, "long", "alpha", strings.Repeat("b", 500), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.SpillPath == second.SpillPath || first.SpillPath == "" || second.SpillPath == "" {
		t.Fatalf("spill paths collided: %q %q", first.SpillPath, second.SpillPath)
	}
	for path, want := range map[string]byte{first.SpillPath: 'a', second.SpillPath: 'b'} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) != 501 || raw[0] != want {
			t.Fatalf("spill %s was overwritten: len=%d head=%q", path, len(raw), raw[:1])
		}
	}
}
