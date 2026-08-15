package seat

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRolloutCommentaryDoesNotCompleteASeat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	content := "" +
		`{"type":"event_msg","payload":{"type":"task_started","turn_id":"turn"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"I'll do that."}]}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := inspectRolloutTurn(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if state != rolloutTurnPending {
		t.Fatalf("commentary state = %q, want pending", state)
	}
}

func TestRolloutRequiresExplicitCompleteWithFinalMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	content := "" +
		`{"type":"event_msg","payload":{"type":"task_started","turn_id":"turn"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"turn","last_agent_message":"finished"}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := inspectRolloutTurn(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if state != rolloutTurnComplete {
		t.Fatalf("completed state = %q", state)
	}

	content = "" +
		`{"type":"event_msg","payload":{"type":"task_started","turn_id":"turn"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"turn","last_agent_message":""}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectRolloutTurn(context.Background(), path); err == nil {
		t.Fatal("task_complete without a final assistant message passed")
	}
}

func TestRolloutBindsEffectiveModelAndEffortToActiveTurn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	content := "" +
		`{"type":"event_msg","payload":{"type":"task_started","turn_id":"old"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"old","model":"wrong","effort":"low"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"old","last_agent_message":"old"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_started","turn_id":"active"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"unrelated","model":"wrong","effort":"low"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"active","model":"gpt-5.6-luna","effort":"xhigh"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"active","last_agent_message":"done"}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	evidence, err := inspectRolloutTurnEvidence(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.State != rolloutTurnComplete || !evidence.Started ||
		evidence.Model != SeatModel || evidence.Effort != SeatEffort {
		t.Fatalf("active turn evidence = %#v", evidence)
	}
}

func TestRolloutAbortedTurnFailsClosedAndPartialTailIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	content := "" +
		`{"type":"event_msg","payload":{"type":"task_started","turn_id":"turn"}}` + "\n" +
		`{"type":"response_item"` // The live writer has not ended this line.
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := inspectRolloutTurn(context.Background(), path)
	if err != nil || state != rolloutTurnPending {
		t.Fatalf("partial live tail = %q, %v", state, err)
	}

	content = "" +
		`{"type":"event_msg","payload":{"type":"task_started","turn_id":"turn"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"turn_aborted","turn_id":"turn"}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err = inspectRolloutTurn(context.Background(), path)
	if err != nil || state != rolloutTurnAborted {
		t.Fatalf("aborted state = %q, %v", state, err)
	}
}
