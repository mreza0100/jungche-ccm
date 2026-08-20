package mcpserv

import (
	"context"
	"io"
	"path/filepath"
	"reflect"
	"testing"

	"hostops/pfm/internal/paths"
)

func TestChatMCPUsesCLISharedOperationsForListFindRead(t *testing.T) {
	setupBackendFixture(t)
	resolved, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	service, err := NewConfigured("test", nil, Runtime{
		Paths: resolved,
		Operations: SharedOperations{
			List: func(_ context.Context, input LSInput) (LSOutput, error) {
				calls = append(calls, "ls:"+input.Project)
				return LSOutput{Count: 11}, nil
			},
			Find: func(_ context.Context, input FindInput) (FindOutput, error) {
				calls = append(calls, "find:"+input.Excerpt)
				return FindOutput{Count: 12}, nil
			},
			Read: func(_ context.Context, input ReadInput) (ReadOutput, error) {
				calls = append(calls, "read:"+input.Source)
				return ReadOutput{Count: 13}, nil
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	_, ls, err := service.chatLS(context.Background(), nil, LSInput{Project: "alpha"})
	if err != nil || ls.Count != 11 {
		t.Fatalf("chat_ls = %+v, err=%v", ls, err)
	}
	_, find, err := service.chatFind(context.Background(), nil, FindInput{Excerpt: "excerpt"})
	if err != nil || find.Count != 12 {
		t.Fatalf("chat_find = %+v, err=%v", find, err)
	}
	_, read, err := service.chatRead(context.Background(), nil, ReadInput{Source: "source"})
	if err != nil || read.Count != 13 {
		t.Fatalf("chat_read = %+v, err=%v", read, err)
	}
	if want := []string{"ls:alpha", "find:excerpt", "read:source"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("shared operation calls = %q, want %q", calls, want)
	}
}

func TestChatMCPDispatchesStatefulActionsInProcess(t *testing.T) {
	setupBackendFixture(t)
	resolved, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	service, err := NewConfigured("test", nil, Runtime{
		Paths: resolved,
		Dispatch: func(_ context.Context, args []string, _ io.Writer, _ io.Writer) int {
			got = append(got, filepath.Join(args...))
			return 0
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	if _, _, err := service.chatName(context.Background(), nil, NameInput{
		Target: "target", Name: "new name",
	}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"chat/name/target/new name"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("dispatch args = %q, want %q", got, want)
	}
}
