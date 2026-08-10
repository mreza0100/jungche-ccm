package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"hostops/cc-fleet/internal/inject"
)

// steerList collects a repeated --steer flag in the order it was given, so a
// chain delivers in the order the caller wrote it.
type steerList []string

func (list *steerList) String() string {
	return fmt.Sprintf("%v", []string(*list))
}

func (list *steerList) Set(value string) error {
	if value == "" {
		return fmt.Errorf("a then steer must be non-empty")
	}
	*list = append(*list, value)
	return nil
}

// runInternalThen is the detached waiter behind chat_inject's `then`
// argument, mirroring chat.sh's __then subcommand: it rides out the primary
// turn and delivers the first steer once the pane has settled to idle,
// carrying the remainder so the chain re-arms one confirmed delivery at a
// time. It is spawned detached because a self-inject's waiter waits on the
// very turn that spawned it.
func runInternalThen(args []string, stderr io.Writer) int {
	flags := newFlagSet(
		"internal then",
		"usage: cc-fleet internal then --socket path --target name --steer text [--steer text]...",
		stderr,
	)
	socket := flags.String("socket", "", "tmux socket path of the target")
	target := flags.String("target", "", "tmux session name or pane id")
	var steers steerList
	flags.Var(&steers, "steer", "follow-up steer; repeat for a chain")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || *target == "" || len(steers) == 0 {
		flags.Usage()
		return 2
	}
	engine, err := inject.New(inject.Dependencies{})
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet internal then: %v\n", err)
		return 1
	}
	result, err := engine.DeliverThen(
		context.Background(),
		*socket,
		*target,
		steers,
	)
	if err != nil {
		fmt.Fprintf(stderr, "cc-fleet internal then: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "then steer -> %s (code %d): %s\n",
		*target,
		result.Code,
		result.Message,
	)
	return result.Code
}

var _ flag.Value = (*steerList)(nil)
