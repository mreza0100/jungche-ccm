# Remote OAuth conformance

Audited 2026-07-26 against the three MCP authorization revisions supported by Claude and the
underlying OAuth discovery/registration/resource specifications. “Served today” describes the
authenticated external application returned by `harvester.remote.build_app`; the unauthenticated
loopback application is covered separately and intentionally publishes none of these routes.

Primary sources:

- [MCP authorization 2025-03-26](https://modelcontextprotocol.io/specification/2025-03-26/basic/authorization)
- [MCP authorization 2025-06-18](https://modelcontextprotocol.io/specification/2025-06-18/basic/authorization)
- [MCP authorization 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization)
- [Anthropic connector authentication](https://claude.com/docs/connectors/building/authentication)
  and [custom connector overview](https://claude.com/docs/connectors/building)
- [RFC 9728](https://www.rfc-editor.org/rfc/rfc9728.html),
  [RFC 8414](https://www.rfc-editor.org/rfc/rfc8414.html),
  [RFC 7591](https://www.rfc-editor.org/rfc/rfc7591.html),
  [RFC 8707](https://www.rfc-editor.org/rfc/rfc8707.html),
  [RFC 6749](https://www.rfc-editor.org/rfc/rfc6749.html), and
  [RFC 7009](https://www.rfc-editor.org/rfc/rfc7009.html)
- [OpenID Connect Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0.html)

## Coverage table

| Requirement | Source (spec + section) | Path/behaviour | Served today? | Verified how |
|---|---|---|---|---|
| MCP transport is served at the resource URI without redirecting to a slash variant | RFC 9728 §3.3; Anthropic “Cross-host authorization servers” | `POST /mcp` is handled at exactly `/mcp`; metadata `resource` is exactly the URL entered by the user | YES | `test_mcp_served_at_exact_path_without_redirect`; `test_protected_resource_metadata_matches_entered_url` |
| An unauthenticated protected-resource request returns 401, not an MCP tool error | MCP 2025-03-26 “Example: authorization code grant”; MCP 2025-06-18 “Authorization Server Location” | `POST /mcp` without a valid bearer returns 401 | YES | `test_401_carries_resource_metadata_pointer` |
| The 401 bearer challenge points to protected-resource metadata | RFC 9728 §5.1; MCP 2025-06-18 “Authorization Server Location”; Anthropic “Cross-host authorization servers” | `WWW-Authenticate: Bearer … resource_metadata="https://…/.well-known/oauth-protected-resource/mcp"` | YES | `test_401_carries_resource_metadata_pointer` |
| The initial challenge advertises the required scope | MCP 2025-11-25 “Protected Resource Metadata Discovery Requirements”; Anthropic “DCR and CIMD details” | `WWW-Authenticate` includes `scope="harvest"` | YES | `test_401_carries_resource_metadata_pointer` |
| Path-derived protected-resource metadata | RFC 9728 §§3, 3.1; MCP 2025-11-25 “Protected Resource Metadata Discovery Requirements” | `GET /.well-known/oauth-protected-resource/mcp` | YES | `test_protected_resource_metadata_matches_entered_url` |
| Root protected-resource fallback | MCP 2025-11-25 “Protected Resource Metadata Discovery Requirements”; Anthropic “Cross-host authorization servers” | `GET /.well-known/oauth-protected-resource` | YES, compatibility alias | `test_bare_protected_resource_metadata_is_served` |
| Protected-resource aliases cannot drift | Deployment robustness requirement in the builder brief | Bare and `/mcp` paths use the same `Route.endpoint` and emit byte-identical bodies | YES | `test_bare_and_suffixed_protected_resource_metadata_cannot_drift` |
| Protected-resource response status and media type | RFC 9728 §3.2 | 200 with `Content-Type: application/json` | YES | `test_every_metadata_alias_is_json_and_cors_readable` |
| Protected-resource identity is exact | RFC 9728 §3.3; Anthropic “Cross-host authorization servers” | `resource` is `https://…/mcp`, including the path | YES | `test_protected_resource_metadata_matches_entered_url` |
| Protected-resource document identifies an authorization server | RFC 9728 §2; MCP 2025-06-18 “Authorization Server Location” | `authorization_servers` contains the root issuer | YES | `test_protected_resource_metadata_matches_entered_url` |
| Protected-resource scope and bearer method are honest | RFC 9728 §2; MCP 2025-11-25 “Scope Selection Strategy” | `scopes_supported=["harvest"]`; `bearer_methods_supported=["header"]` | YES | Metadata response assertion plus SDK model/source inspection |
| RFC 8414 discovery for a root issuer | RFC 8414 §§3, 3.1; MCP 2025-03-26 “Server Metadata Discovery”; MCP 2025-06-18 “Server Metadata Discovery” | `GET /.well-known/oauth-authorization-server` | YES | `test_authorization_server_metadata_advertises_s256_and_dcr` |
| RFC 8414 path-insertion probe based on the MCP path | RFC 8414 §3.1; MCP 2025-11-25 “Authorization Server Metadata Discovery” | `GET /.well-known/oauth-authorization-server/mcp` | YES, compatibility alias; the advertised issuer itself is root | `test_suffixed_authorization_server_metadata_matches_root` |
| OpenID discovery root probe | MCP 2025-11-25 “Authorization Server Metadata Discovery”; Anthropic “Cross-host authorization servers”; RFC 8414 §5 | `GET /.well-known/openid-configuration` serves the same OAuth metadata | YES, compatibility alias | `test_openid_configuration_matches_authorization_server_metadata` |
| OpenID path-insertion probe | MCP 2025-11-25 “Authorization Server Metadata Discovery”; RFC 8414 §5 | `GET /.well-known/openid-configuration/mcp` serves the same OAuth metadata | YES, compatibility alias | `test_openid_configuration_suffixed_matches_authorization_server_metadata` |
| OpenID path-appending probe for an issuer containing a path | OIDC Discovery §4.1; MCP 2025-11-25 “Authorization Server Metadata Discovery” | `/mcp/.well-known/openid-configuration` | N/A / SKIPPED | The advertised issuer is `https://host/` with no path, so a compliant client does not construct this location. Serving the root-issuer document there would fail OIDC’s exact-issuer validation. |
| Discovery documents are browser-readable | OIDC Discovery §4; connector/browser interoperability | All six served metadata locations answer cross-origin GET with `Access-Control-Allow-Origin: *` | YES | `test_every_metadata_alias_is_json_and_cors_readable` |
| Authorization-server aliases cannot drift | Deployment robustness requirement in the builder brief | RFC 8414 and both OpenID aliases reuse one endpoint and emit byte-identical bodies | YES | Three byte-comparison tests for RFC 8414 and OpenID aliases |
| Authorization-server metadata has an exact issuer and real endpoints | RFC 8414 §2 | Root issuer; `/authorize`, `/token`, `/register`, and `/revoke` URLs match mounted routes | YES | Metadata assertions and route-level flow tests |
| Authorization code and refresh grants are advertised honestly | RFC 8414 §2 | `grant_types_supported=["authorization_code","refresh_token"]`; `response_types_supported=["code"]` | YES | Metadata response inspection; full code and refresh tests |
| S256 PKCE is advertised | MCP 2025-11-25 “Authorization Code Protection”; Anthropic “DCR and CIMD details” | `code_challenge_methods_supported=["S256"]` on every discovery alias | YES | `test_authorization_server_metadata_advertises_s256_and_dcr`; alias byte tests |
| Token client-auth metadata matches accepted methods | RFC 8414 §2; RFC 7591 §2; Anthropic “DCR and CIMD details” | `token_endpoint_auth_methods_supported` is `none`, `client_secret_post`, `client_secret_basic` | YES | `test_authorization_server_metadata_advertises_every_accepted_client_auth_method`; public and Basic full-flow tests; SDK source inspection for POST |
| Revocation client-auth metadata matches accepted methods | RFC 7009 §2.1 plus registered RFC 8414 metadata | `revocation_endpoint_auth_methods_supported` contains the same three methods | YES | Metadata assertion; public revocation test; SDK authenticator inspection |
| Authorization-server scopes are honest | RFC 8414 §2; MCP 2025-11-25 “Scope Selection Strategy” | `scopes_supported=["harvest"]` | YES | `test_authorization_server_metadata_advertises_s256_and_dcr` |
| `offline_access` is advertised only if it is a real scope | Anthropic “DCR and CIMD details”; OIDC Core §11 | It is not advertised or accepted; this OAuth server returns refresh tokens for the authorization-code grant without relying on the OIDC-only scope | YES | Metadata assertion and full flow showing a refresh token without `offline_access` |
| Authorization endpoint supports the code flow | RFC 6749 §4.1.1; MCP all revisions | `GET` and form `POST /authorize`, followed by `/consent` and an exact registered redirect | YES | Full-flow tests; SDK route/source inspection for POST |
| Authorization redirects are exact and pre-registered | MCP 2025-11-25 “Open Redirection”; OAuth 2.1 security requirements | Request URI is compared against DCR metadata before any redirect | YES | SDK `validate_redirect_uri` source inspection; invalid DCR redirect tests |
| Redirects use HTTPS or HTTP loopback and have no fragment | MCP 2025-03-26 “Security Considerations”; MCP 2025-06-18/2025-11-25 “Communication Security” | DCR rejects insecure non-loopback HTTP and fragments; accepts HTTPS and loopback HTTP | YES | `test_registration_rejects_metadata_the_server_cannot_honor`; `test_http_loopback_redirect_is_allowed_for_native_clients` |
| `state` is preserved | RFC 6749 §§4.1.1, 4.1.2 | Success and authorization errors return the request state | YES | `test_full_authorization_code_flow`; `test_authorize_rejects_a_resource_for_another_server` |
| PKCE is mandatory for all registered clients | MCP 2025-03-26 “Implementation Requirements”; MCP 2025-11-25 “Authorization Code Protection” | `/authorize` requires `code_challenge` and only `S256`; malformed challenges are rejected | YES | SDK request model plus `test_pkce_rejects_a_malformed_challenge` |
| PKCE verifier is enforced at exchange | RFC 7636 §4.6; OAuth 2.1; MCP all revisions | `/token` hashes the verifier and returns `invalid_grant` on mismatch | YES | `test_pkce_rejects_a_wrong_verifier` |
| Authorization `resource` is understood and canonical | RFC 8707 §§2, 2.1; MCP 2025-06-18/2025-11-25 “Resource Parameter Implementation” | Accepts the exact `/mcp` resource and scheme/host case variants; rejects other audiences with `invalid_target` | YES | `test_resource_indicator_is_validated_on_authorize_and_token`; `test_authorize_rejects_a_resource_for_another_server` |
| Pre-resource-indicator MCP clients remain compatible | MCP 2025-03-26; RFC 8707 §2.1 | A missing authorization `resource` defaults to this server’s canonical `/mcp` audience | YES | `test_full_authorization_code_flow` omits `resource`, then successfully uses an audience-validated token |
| Token endpoint accepts form encoding | RFC 6749 §§4.1.3, 6; Anthropic “Token refresh” | `POST /token` requires and accepts `application/x-www-form-urlencoded` | YES | All token/refresh flow tests; `test_token_and_revocation_endpoints_require_form_encoding` rejects JSON |
| Public DCR clients authenticate with `none` | RFC 7591 §2; Anthropic “DCR and CIMD details” | A public client sends `client_id` and no secret | YES | `test_authorization_server_metadata_advertises_every_accepted_client_auth_method` |
| Confidential clients can use standard HTTP Basic | RFC 6749 §2.3.1 | Basic credentials supply the client ID and secret; duplicate `client_id` in the form is not required | YES | `test_confidential_client_can_use_standard_http_basic_token_auth` |
| Confidential clients can use client-secret POST | RFC 6749 §2.3.1 | Form `client_id` and `client_secret` are accepted for clients registered for this method | YES | SDK client-authenticator source inspection; method is constrained and advertised |
| Token `resource` is handled on code and refresh requests | RFC 8707 §2.2; MCP 2025-06-18/2025-11-25 “Resource Parameter Implementation” | Matching `/mcp` is accepted; another or multiple resource is `invalid_target`; a bad target does not burn the code/refresh token | YES | Code and refresh invalid-target tests |
| Access tokens are audience-bound and the resource server validates the audience | MCP 2025-06-18 “Token Handling” and “Token Audience Binding”; MCP 2025-11-25 same | Every OAuth access token, including refreshed and 2025-03-compatible tokens, is bound to canonical `/mcp`; a mismatched/missing OAuth audience is rejected | YES | Matching code/refresh tokens reach `/mcp`; `test_resource_server_rejects_an_access_token_for_another_audience` |
| Authorization codes are short-lived and single-use | OAuth 2.1 authorization-code security; RFC 6749 §10.5 | Five-minute in-memory code; burned before issuing tokens | YES | `test_authorization_code_is_single_use`; provider constant/source inspection for TTL |
| Redirect URI is unchanged at token exchange | RFC 6749 §§4.1.3, 4.1.4 | SDK compares the token request URI with the authorization request | YES | SDK handler source inspection; full-flow pin |
| Token success and errors are non-cacheable JSON | RFC 6749 §§5.1, 5.2 | `application/json`, `Cache-Control: no-store`, `Pragma: no-cache` | YES | SDK response source inspection and flow response inspection |
| Bad/expired/replayed grants use `invalid_grant` | RFC 6749 §5.2; Anthropic “Token refresh” | Wrong verifier, used code, and old refresh token return `invalid_grant` | YES | Three explicit error-code assertions |
| Unsupported grants use `unsupported_grant_type` | RFC 6749 §5.2 | `client_credentials` and unknown grants are rejected with the standard code | YES | `test_token_uses_rfc6749_error_for_unsupported_grant` |
| Refresh tokens rotate for public clients | MCP 2025-06-18/2025-11-25 “Token Theft”; Anthropic “Token refresh” | Old refresh token is removed as the replacement is issued and returned | YES | `test_refresh_token_rotates_and_old_one_dies` |
| Refreshed access tokens remain audience-bound | MCP 2025-06-18/2025-11-25 “Token Audience Binding and Validation” | Refresh exchange always mints for canonical `/mcp` | YES | The refreshed token successfully passes mandatory audience validation in the rotation test |
| DCR accepts JSON | RFC 7591 §§3, 3.1; Anthropic “Token refresh” | `POST /register` requires and accepts `application/json`; malformed/wrong media type returns JSON error rather than raising | YES | `test_registration_requires_json_and_disables_response_caching` |
| DCR success response is complete and non-cacheable | RFC 7591 §3.2.1 | 201 JSON; registered metadata and client ID; secret and `client_secret_expires_at=0` when applicable; no-store/no-cache | YES | Registration default/secret and caching tests |
| DCR’s omitted auth-method default is Basic | RFC 7591 §2 | Missing `token_endpoint_auth_method` becomes `client_secret_basic` | YES | `test_registration_uses_rfc7591_default_and_complete_secret_metadata` |
| DCR does not register an unusable client | RFC 7591 §§2, 3.2.2 | Unsupported `private_key_jwt`, invalid scope/grant combinations, and invalid redirects are rejected with RFC 7591 errors | YES | Parametrized invalid-metadata tests plus SDK scope/grant validation tests/source |
| Revocation endpoint accepts form encoding | RFC 7009 §2.1 | `POST /revoke` requires `application/x-www-form-urlencoded` | YES | Public revocation flow and media-type rejection test |
| Public clients can revoke without a secret | RFC 7009 §2.1; RFC 7591 public-client semantics | `client_id` plus token is sufficient for a client registered with `none` | YES | `test_public_client_can_revoke_without_a_client_secret` |
| Revocation is idempotent and non-cacheable | RFC 7009 §2.2 | Known and unknown tokens return 200; response is no-store/no-cache | YES | Public revocation and unknown-token tests |
| Bearer tokens are header-only | MCP all revisions “Token Requirements”; RFC 6750 §2.1 | `Authorization: Bearer`; query-string access tokens are not parsed | YES | Auth middleware source inspection and bearer tests |
| Invalid/expired bearer tokens return 401; insufficient scope returns 403 | MCP all revisions “Token Handling/Error Handling”; RFC 6750 §3.1 | 401 `invalid_token`; 403 `insufficient_scope` with metadata/scope challenge | YES; 403 branch not externally reachable with this one-scope issuer | Wrong-token tests; middleware source inspection. The 403 shape is wrapped and code-inspected but marked **SKIPPED for end-to-end triggering** because the AS never issues a token lacking its only required scope. |
| Access tokens expire and are not persisted | OAuth 2.1 token security; MCP “Token Theft” | One-hour in-memory access tokens; expiry checked on load | YES | Provider source inspection; state-file test proves access token is absent from disk. **SKIPPED wall-clock wait** to keep tests deterministic. |
| OAuth state survives restart without storing live tokens | OAuth token-storage security | DCR clients and refresh-token digests persist owner-only; codes/access tokens do not | YES | State corruption, live-token absence, owner-only mode tests |
| Authorization endpoints use HTTPS | RFC 8414 §2; MCP communication-security sections | Non-loopback OAuth issuer is rejected unless HTTPS; localhost HTTP remains available for development | YES | SDK issuer validation source inspection; production `PUBLIC` fixture is HTTPS |
| OAuth paths are absent in unauthenticated mode | Authorization is optional in all MCP revisions | Both methods on every metadata, authorization, token, registration, revocation, and consent path return 404 | YES | `test_open_app_publishes_no_oauth_metadata` enumerates every path |
| Static-token-only mode does not advertise an unusable OAuth AS | Honest discovery behavior | `/mcp` requires the configured bearer, but OAuth metadata/consent are absent | YES | `test_static_token_only_requires_bearer_but_mounts_no_oauth` |
| Client ID Metadata Documents (CIMD) | MCP 2025-11-25 “Client ID Metadata Documents”; Anthropic “DCR and CIMD details” | URL client IDs and `client_id_metadata_document_supported` | **SKIPPED / not implemented** | MCP says SHOULD, not MUST, and Claude falls back to the advertised DCR endpoint. Implementing CIMD would add an authorization-server SSRF surface and requires a separate design/audit. Metadata intentionally does not claim support. |
| Full OpenID Provider semantics | OIDC Discovery §§3–4 | ID-token/JWKS/subject-type metadata and an `openid` grant | **SKIPPED / not implemented** | Harvester is an OAuth AS, not an OpenID Provider, and emits no ID tokens. The OpenID-named aliases are RFC 8414 §5 compatibility locations for the same OAuth document; they must not claim nonexistent OP features. |
| Signed metadata | RFC 8414 §2.1; RFC 9728 §2 | `signed_metadata` JWT | **SKIPPED / optional** | Neither RFC nor MCP requires it for this same-origin single-operator deployment. |
| Multiple authorization servers | RFC 9728 §2; MCP 2025-11-25 “Authorization Server Location” | More than one `authorization_servers` entry and client selection | **SKIPPED / not configured** | Harvester has exactly one co-hosted AS. The one required entry is present and first. |
| DPoP, token introspection, PAR, and JWT client authentication | Optional OAuth extensions | Additional endpoints/methods | **SKIPPED / not advertised** | The three MCP base authorization revisions and Anthropic DCR flow do not require them; metadata makes no support claim. |
| Live Claude/browser, public TLS, WAF/egress, and latency | Anthropic “Cross-host authorization servers” and “Endpoint latency” | Deployed ports 8081/8082 and public tunnel | **SKIPPED in this audit** | The brief forbids restarting/touching the live server and says the reviewer will deploy. Local `TestClient` verifies application behavior only; external reachability, certificate validation, Anthropic egress allowlisting, and the 10/30-second production budgets require post-deploy observation. |

## Findings and fixes from this sweep

1. **Public-client discovery was self-contradictory.** DCR accepted `none`, while
   `token_endpoint_auth_methods_supported` omitted it. RFC 8414 §2 says the array lists methods
   supported by the endpoint, and Anthropic requires `none` for its public DCR/CIMD client. Both
   token and revocation metadata now advertise the exact accepted set, and all discovery aliases
   reuse that canonical endpoint.
2. **RFC 8707 was parsed but not honored at `/token`, and refreshed tokens lost their audience.**
   MCP 2025-06-18 and 2025-11-25 require clients to send `resource` at authorization and token
   requests and require the resource server to validate audience. Both endpoints now reject another
   audience with `invalid_target`; all issued/refreshed tokens are canonicalized to `/mcp`; inbound
   OAuth tokens without that audience are rejected. Missing `resource` still defaults safely for
   2025-03-26 compatibility, as RFC 8707 §2.1 permits.
3. **The bearer challenge omitted scope guidance.** MCP 2025-11-25 recommends a `scope` parameter,
   and Anthropic uses it ahead of metadata fallback. The 401/403 challenge now includes `harvest`.
4. **Several SDK endpoint shapes did not match their advertisements.** Standard HTTP Basic failed
   unless `client_id` was duplicated in the form; public revocation failed unless a meaningless
   `client_secret` field was present; unknown grants surfaced `invalid_request`. Thin route wrappers
   now restore RFC 6749/7009 behavior without duplicating the SDK’s token, PKCE, redirect, or client
   authentication implementations.
5. **DCR had RFC 7591 gaps.** Wrong media types could raise, the omitted authentication-method
   default was POST rather than Basic, non-expiring secrets omitted the required zero expiry,
   responses lacked no-store headers, and the server registered `private_key_jwt` clients it could
   never authenticate. Registration now applies RFC defaults/headers, validates redirect security,
   and rejects unsupported metadata with RFC 7591 errors.
6. **PKCE advertisement and enforcement were not equally strict.** S256 was advertised and verifier
   checking existed, but malformed challenges could be parked for consent. Challenges are now
   validated before consent.

## Explicitly not verified

The SKIPPED rows above are part of the coverage result, not omissions. In particular, this work did
not restart either live gateway, read or modify `.env.remote`, make a real Claude connector attempt,
validate the public certificate/tunnel/WAF, measure Anthropic-facing latency, wait an hour for a real
access-token expiry, synthesize an unreachable insufficient-scope grant, or implement optional
CIMD/OpenID Provider/signed-metadata/DPoP features. Those limits are intentional and stated in the
table so an empty future finding list cannot hide unexamined behavior.
