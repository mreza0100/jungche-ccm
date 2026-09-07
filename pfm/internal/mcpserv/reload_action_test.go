package mcpserv

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"hostops/pfm/internal/inject"
)

// fakeReloadInjector answers Resolve deterministically for the chatReload
// regression below. Every other injectionService method panics if called —
// chatReload has no business reaching them.
type fakeReloadInjector struct {
	target inject.Target
	code   int
	detail string
	err    error
}

func (fake fakeReloadInjector) Resolve(_ context.Context, _ string) (inject.Target, int, string, error) {
	return fake.target, fake.code, fake.detail, fake.err
}

func (fake fakeReloadInjector) ResolveEngine(context.Context, string, string) (inject.Target, int, string, error) {
	panic("chatReload must not call ResolveEngine")
}

func (fake fakeReloadInjector) Capture(context.Context, string, int) (inject.Target, string, int, string, error) {
	panic("chatReload must not call Capture")
}

func (fake fakeReloadInjector) Inject(context.Context, inject.Request) (inject.Result, error) {
	panic("chatReload must not call Inject")
}

func (fake fakeReloadInjector) ScheduleAfterCurrentTurn(context.Context, inject.Request) (inject.Result, error) {
	panic("chatReload must not call ScheduleAfterCurrentTurn")
}

func (fake fakeReloadInjector) ScheduleSelfCompact(context.Context, string, []string) (inject.Result, error) {
	panic("chatReload must not call ScheduleSelfCompact")
}

// TestChatReloadDispatchesSockNotBareTarget is the F2 regression: chatReload
// used to forward the caller's bare target straight through as a positional
// argument ("chat", "reload", target), but `pfm chat reload` takes no
// positional target at all — it rejects every string it is given with
// "%q is not a reload argument". The fix resolves the target through the
// injector and dispatches the socket it names with --sock, which is the only
// spelling `pfm chat reload` accepts.
func TestChatReloadDispatchesSockNotBareTarget(t *testing.T) {
	var calls [][]string
	service := &Service{backend: &backend{
		injector: fakeReloadInjector{
			target: inject.Target{SocketPath: "/jailed/tmux/cc-1234-5-6"},
		},
		dispatch: func(_ context.Context, args []string, _ io.Writer, _ io.Writer) int {
			calls = append(calls, append([]string(nil), args...))
			return 0
		},
	}}

	if _, _, err := service.chatReload(
		context.Background(), nil, TargetInput{Target: "some-chat-name"},
	); err != nil {
		t.Fatalf("chatReload: %v", err)
	}
	want := [][]string{{"chat", "reload", "--sock", "cc-1234-5-6"}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("dispatch calls = %q, want %q", calls, want)
	}
}

// TestChatReloadRefusesATargetThatResolvesToNoSocket covers the boundary the
// fix added on top of the dispatch-shape fix: a target the injector cannot
// place on a live tmux socket must fail loudly rather than dispatch a
// meaningless "--sock ." or "--sock /" — only a LIVE chat can be reloaded.
func TestChatReloadRefusesATargetThatResolvesToNoSocket(t *testing.T) {
	dispatched := false
	service := &Service{backend: &backend{
		injector: fakeReloadInjector{target: inject.Target{}},
		dispatch: func(context.Context, []string, io.Writer, io.Writer) int {
			dispatched = true
			return 0
		},
	}}

	if _, _, err := service.chatReload(
		context.Background(), nil, TargetInput{Target: "resumable-only"},
	); err == nil || !strings.Contains(err.Error(), "no tmux socket") {
		t.Fatalf("chatReload error = %v, want a no-tmux-socket refusal", err)
	}
	if dispatched {
		t.Fatal("chatReload dispatched despite an unresolved socket")
	}
}

// TestChatReloadPropagatesResolveFailure is a narrow companion: a resolve
// error (ambiguous target, resolver failure) must surface as an error too,
// not be swallowed into a silent no-op dispatch.
func TestChatReloadPropagatesResolveFailure(t *testing.T) {
	service := &Service{backend: &backend{
		injector: fakeReloadInjector{code: 2, detail: "ambiguous", err: errors.New("boom")},
		dispatch: func(context.Context, []string, io.Writer, io.Writer) int {
			t.Fatal("chatReload dispatched despite a resolve error")
			return 0
		},
	}}
	if _, _, err := service.chatReload(
		context.Background(), nil, TargetInput{Target: "whatever"},
	); err == nil {
		t.Fatal("chatReload error = nil, want the wrapped resolve error")
	}
}
