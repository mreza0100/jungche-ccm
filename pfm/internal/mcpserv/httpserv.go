package mcpserv

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewHTTPHandler exposes the same server and tool registrations as the stdio
// transport through MCP streamable HTTP. Authentication and endpoint routing
// belong to the process-level daemon, so this handler is also useful in
// httptest and in a caller that mounts it under its own policy.
func (service *Service) NewHTTPHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return service.server },
		&mcp.StreamableHTTPOptions{
			JSONResponse:               true,
			Stateless:                  false,
			DisableLocalhostProtection: false,
		},
	)
}
