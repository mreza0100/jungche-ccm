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
}
