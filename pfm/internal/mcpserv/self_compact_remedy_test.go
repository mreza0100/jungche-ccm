package mcpserv

import (
	"strings"
	"testing"

	"hostops/pfm/internal/inject"
)

// chat_self_compact has no `pfm chat` CLI twin, so the generic
// ambient-identity remedy — "run the equivalent `pfm chat ...` command" — is
// a dangling pointer exactly where the caller needs a working next step. Its
// refusal must name the real fallback: an inject into the chat's own pane.
func TestSelfCompactAmbientRefusalNamesAWorkingRemedy(t *testing.T) {
	service := metadataIdentityService(t)
	protocol := connectInMemory(t, service.Server())
	refused := callToolWithMeta[InjectOutput](
		t, protocol.clientSession, "chat_self_compact", nil,
		map[string]any{"focus": "hold the goal", "then": []string{"resume the wave"}},
	)
	if refused.Status != "not_found" || refused.Code != inject.CodeUnknown {
		t.Fatalf("metadata-free chat_self_compact = %+v", refused)
	}
	if !strings.Contains(refused.Message, "pfm chat inject") {
		t.Fatalf("refusal does not name the inject fallback: %q", refused.Message)
	}
	if strings.Contains(refused.Message, "equivalent `pfm chat ...` command") {
		t.Fatalf("refusal still claims a CLI twin exists: %q", refused.Message)
	}
	// stdlib flag parsing stops at the first positional — every flag must
	// precede the <target> placeholder, or it folds into the message body.
	command := "pfm chat inject "
	commandStart := strings.Index(refused.Message, command)
	if commandStart == -1 {
		t.Fatalf("refusal does not contain the inject command: %q", refused.Message)
	}
	flagsIndex := strings.Index(refused.Message[commandStart:], "--force-now")
	thenIndex := strings.Index(refused.Message[commandStart:], "--then")
	targetIndex := strings.Index(refused.Message[commandStart:], "$(pfm whoami)")
	if flagsIndex == -1 || thenIndex == -1 || targetIndex == -1 {
		t.Fatalf("refusal is missing an expected token: %q", refused.Message)
	}
	if !(flagsIndex < targetIndex && thenIndex < targetIndex) {
		t.Fatalf("refusal places a flag after the target, breaking stdlib flag parsing: %q", refused.Message)
	}
}
