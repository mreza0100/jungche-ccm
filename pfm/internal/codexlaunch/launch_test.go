package codexlaunch

import (
	"context"
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrepareAppendsAcrossModelsWithoutReplacingBase(t *testing.T) {
	home := t.TempDir()
	file := PromptPath(home)
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("Professor appendix. God speed."), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"model-one", "model-two"} {
		args := []string{"--model", model, "exec", "-"}
		got, err := Prepare(context.Background(), "unused", home, args, func(context.Context, string, []string) (string, error) { return "Keep personal instructions.", nil })
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(got, " ")
		if !strings.Contains(joined, "Professor appendix. God speed.") || !strings.Contains(joined, "Keep personal instructions.") {
			t.Fatalf("missing appended instructions: %q", got)
		}
		if !strings.Contains(joined, model) || strings.Contains(joined, "model_instructions_file") {
			t.Fatalf("base model was replaced: %q", got)
		}
	}
}

func TestPreparePreservesCallerArgumentsAndTOMLText(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(PromptPath(home)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PromptPath(home), []byte("Appendix 🍵\nquote: \"yes\""), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-c", "model_instructions_file='/base.md'", "exec", "--", "literal -c prompt"}
	got, err := Prepare(context.Background(), "binary", home, args, func(_ context.Context, _ string, passed []string) (string, error) {
		if !reflect.DeepEqual(args, passed) {
			t.Fatal("reader did not receive caller overrides")
		}
		return "    Personal\n\\path\t☕", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := append(append([]string{}, args[:3]...), "-c", got[4], "--", "literal -c prompt")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args changed: %q", got)
	}
	var document map[string]any
	if _, err := toml.Decode(got[4], &document); err != nil {
		t.Fatal(err)
	}
	if document["developer_instructions"] != "    Personal\n\\path\t☕\n\n"+marker+"\n\nAppendix 🍵\nquote: \"yes\"" {
		t.Fatalf("wrong merged text: %#v", document)
	}
}
func TestPrepareRefusesMissingAppendixAndConfigFailure(t *testing.T) {
	home := t.TempDir()
	read := func(context.Context, string, []string) (string, error) {
		return "", fmt.Errorf("invalid personal config")
	}
	if _, err := Prepare(context.Background(), "binary", home, nil, read); err == nil {
		t.Fatal("missing appendix accepted")
	}
	if err := os.MkdirAll(filepath.Dir(PromptPath(home)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PromptPath(home), []byte("Appendix"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), "binary", home, nil, read); err == nil || !strings.Contains(err.Error(), "invalid personal config") {
		t.Fatalf("config failure hidden: %v", err)
	}
}
