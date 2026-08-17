"""Remote transport: the same MCP server over Streamable HTTP, for claude.ai custom connectors.

`serve` (stdio) and `build_app` (HTTP) run the SAME `Server` instance from `server.build_server` —
tool behaviour never diverges between local and remote. What differs is the threat model, and this
module is where that difference is expressed:

  * **Local reads are confined to the cache root.** `fetchImage` and `archive` hand back cache paths
    for the model to read, so local paths must keep working; but a remote caller has no business
    reading the rest of the disk. `detect.set_local_roots(cache_root())` makes everything outside
    the cache unreachable by construction rather than by denylist.
  * **Auth is optional but never accidental.** With no passphrase and no static token the server
    runs open, which is correct for a loopback/tailnet-internal instance. Binding an unauthenticated
    server to a non-loopback address requires an explicit flag — an open fetch proxy on a public
    interface should be a decision, not a default.
  * **Host validation is explicit.** The SDK's DNS-rebinding protection defaults to ON with an EMPTY
    allowlist, which rejects every request. The public hostname is registered here, or nothing works.
"""

import asyncio
import base64
import binascii
import contextlib
import json
import os
from collections.abc import AsyncIterator
from pathlib import Path
from urllib.parse import parse_qs, quote_plus, unquote, urlparse

from mcp.server.auth.handlers.metadata import MetadataHandler
from mcp.server.auth.middleware.bearer_auth import BearerAuthBackend, RequireAuthMiddleware
from mcp.server.auth.provider import construct_redirect_uri
from mcp.server.auth.routes import (
    build_metadata,
    build_resource_metadata_url,
    cors_middleware,
    create_auth_routes,
    create_protected_resource_routes,
)
from mcp.server.auth.settings import ClientRegistrationOptions, RevocationOptions
from mcp.server.streamable_http_manager import StreamableHTTPSessionManager
from mcp.server.transport_security import TransportSecuritySettings
from mcp.shared.auth import InvalidRedirectUriError
from pydantic import AnyHttpUrl, AnyUrl
from starlette.applications import Starlette
from starlette.datastructures import Headers, MutableHeaders
from starlette.middleware import Middleware
from starlette.middleware.authentication import AuthenticationMiddleware
from starlette.requests import Request
from starlette.responses import HTMLResponse, JSONResponse, RedirectResponse, Response
from starlette.routing import Route
from starlette.types import ASGIApp, Message, Receive, Scope, Send

from . import detect
from .auth import SCOPE, HarvesterAuthProvider, resource_matches
from .cache import cache_root
from .log import get_logger
from .server import build_server

log = get_logger("remote")

MCP_PATH = "/mcp"
MCP_METHODS = ["GET", "POST", "DELETE"]   # Streamable HTTP uses all three on the one endpoint


class _ASGIEndpoint:
    """Adapter that serves a raw ASGI app at an EXACT path.

    `Mount("/mcp", …)` does not match `/mcp` itself — it answers with a 307 to `/mcp/`. Claude
    compares the protected-resource `resource` field against the URL the user typed, so the
    endpoint has to live AT `/mcp`, not one redirect away. Starlette's `Route` treats a
    non-function endpoint as a raw ASGI app, so a class instance puts it exactly there.
    """

    def __init__(self, app) -> None:
        self.app = app

    async def __call__(self, scope, receive, send) -> None:
        await self.app(scope, receive, send)


async def _read_body(receive: Receive) -> bytes:
    chunks: list[bytes] = []
    more_body = True
    while more_body:
        message = await receive()
        if message["type"] != "http.request":
            break
        chunks.append(message.get("body", b""))
        more_body = message.get("more_body", False)
    return b"".join(chunks)


def _replay_body(body: bytes) -> Receive:
    sent = False

    async def receive() -> Message:
        nonlocal sent
        if sent:
            return {"type": "http.request", "body": b"", "more_body": False}
        sent = True
        return {"type": "http.request", "body": body, "more_body": False}

    return receive


def _content_type(scope: Scope) -> str:
    return Headers(scope=scope).get("content-type", "").partition(";")[0].strip().lower()


async def _oauth_error(
    scope: Scope,
    receive: Receive,
    send: Send,
    error: str,
    description: str,
) -> None:
    response = JSONResponse(
        {"error": error, "error_description": description},
        status_code=400,
        headers={
            "Cache-Control": "no-store",
            "Pragma": "no-cache",
            "Access-Control-Allow-Origin": "*",
        },
    )
    await response(scope, receive, send)


def _append_form_field(body: bytes, name: str, value: str) -> bytes:
    separator = b"&" if body else b""
    return body + separator + f"{quote_plus(name)}={quote_plus(value)}".encode()


def _basic_client_id(scope: Scope) -> str | None:
    authorization = Headers(scope=scope).get("authorization", "")
    if not authorization.startswith("Basic "):
        return None
    try:
        decoded = base64.b64decode(authorization[6:], validate=True).decode()
        client_id, _separator, _secret = decoded.partition(":")
        return unquote(client_id) or None
    except (ValueError, UnicodeDecodeError, binascii.Error):
        return None


class _TokenConformanceMiddleware:
    """Fill SDK gaps around form encoding, Basic auth, errors, and RFC 8707."""

    def __init__(self, app: ASGIApp, resource_url: str) -> None:
        self.app = app
        self.resource_url = resource_url

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http" or scope["method"] != "POST":
            await self.app(scope, receive, send)
            return
        if _content_type(scope) != "application/x-www-form-urlencoded":
            await _oauth_error(
                scope, receive, send, "invalid_request",
                "token requests must use application/x-www-form-urlencoded",
            )
            return
        body = await _read_body(receive)
        try:
            params = parse_qs(body.decode(), keep_blank_values=True)
        except UnicodeDecodeError:
            await _oauth_error(scope, receive, send, "invalid_request", "request body is not UTF-8")
            return

        grant_types = params.get("grant_type", [])
        if len(grant_types) == 1 and grant_types[0] not in {
            "authorization_code", "refresh_token",
        }:
            await _oauth_error(
                scope, receive, send, "unsupported_grant_type", "grant_type is not supported"
            )
            return

        resources = params.get("resource", [])
        if len(resources) > 1 or (
            resources and not resource_matches(resources[0], self.resource_url)
        ):
            await _oauth_error(
                scope, receive, send, "invalid_target",
                "resource does not identify this MCP server",
            )
            return

        if "client_id" not in params:
            client_id = _basic_client_id(scope)
            if client_id:
                body = _append_form_field(body, "client_id", client_id)
        await self.app(scope, _replay_body(body), send)


class _AuthorizationResourceMiddleware:
    """Validate RFC 8707 before the SDK narrows authorization errors to RFC 6749."""

    def __init__(
        self,
        app: ASGIApp,
        provider: HarvesterAuthProvider,
        resource_url: str,
    ) -> None:
        self.app = app
        self.provider = provider
        self.resource_url = resource_url

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http" or scope["method"] not in {"GET", "POST"}:
            await self.app(scope, receive, send)
            return
        body: bytes | None = None
        if scope["method"] == "GET":
            encoded_params = scope.get("query_string", b"")
        else:
            body = await _read_body(receive)
            encoded_params = body
        try:
            params = parse_qs(encoded_params.decode(), keep_blank_values=True)
        except UnicodeDecodeError:
            await _oauth_error(scope, receive, send, "invalid_request", "request is not UTF-8")
            return
        resources = params.get("resource", [])
        if len(resources) <= 1 and (
            not resources or resource_matches(resources[0], self.resource_url)
        ):
            await self.app(scope, _replay_body(body) if body is not None else receive, send)
            return

        error_fields = {
            "error": "invalid_target",
            "error_description": "resource does not identify this MCP server",
        }
        states = params.get("state", [])
        if len(states) == 1:
            error_fields["state"] = states[0]
        client_ids = params.get("client_id", [])
        redirects = params.get("redirect_uri", [])
        client = await self.provider.get_client(client_ids[0]) if len(client_ids) == 1 else None
        if client is not None and len(redirects) <= 1:
            try:
                requested_redirect = AnyUrl(redirects[0]) if redirects else None
                redirect_uri = client.validate_redirect_uri(requested_redirect)
            except (ValueError, InvalidRedirectUriError):
                pass
            else:
                response = RedirectResponse(
                    construct_redirect_uri(str(redirect_uri), **error_fields),
                    status_code=302,
                    headers={"Cache-Control": "no-store"},
                )
                await response(scope, receive, send)
                return
        await _oauth_error(
            scope, receive, send, error_fields["error"], error_fields["error_description"]
        )


class _RegistrationConformanceMiddleware:
    """Apply RFC 7591 JSON/default/cache requirements around the SDK handler."""

    def __init__(self, app: ASGIApp) -> None:
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http" or scope["method"] != "POST":
            await self.app(scope, receive, send)
            return
        if _content_type(scope) != "application/json":
            await _oauth_error(
                scope, receive, send, "invalid_client_metadata",
                "registration requests must use application/json",
            )
            return
        body = await _read_body(receive)
        try:
            document = json.loads(body)
        except (UnicodeDecodeError, json.JSONDecodeError):
            await _oauth_error(
                scope, receive, send, "invalid_client_metadata",
                "registration body is not valid JSON",
            )
            return
        if isinstance(document, dict) and "token_endpoint_auth_method" not in document:
            # RFC 7591 §2 defines client_secret_basic as the registration default.
            document["token_endpoint_auth_method"] = "client_secret_basic"
            body = json.dumps(document, separators=(",", ":")).encode()

        async def send_no_store(message: Message) -> None:
            if message["type"] == "http.response.start":
                headers = MutableHeaders(scope=message)
                headers["Cache-Control"] = "no-store"
                headers["Pragma"] = "no-cache"
            await send(message)

        await self.app(scope, _replay_body(body), send_no_store)


class _RevocationConformanceMiddleware:
    """Make public and HTTP-Basic clients interoperable with the SDK revocation model."""

    def __init__(self, app: ASGIApp) -> None:
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http" or scope["method"] != "POST":
            await self.app(scope, receive, send)
            return
        if _content_type(scope) != "application/x-www-form-urlencoded":
            await _oauth_error(
                scope, receive, send, "invalid_request",
                "revocation requests must use application/x-www-form-urlencoded",
            )
            return
        body = await _read_body(receive)
        try:
            params = parse_qs(body.decode(), keep_blank_values=True)
        except UnicodeDecodeError:
            await _oauth_error(scope, receive, send, "invalid_request", "request body is not UTF-8")
            return
        if "client_id" not in params:
            client_id = _basic_client_id(scope)
            if client_id:
                body = _append_form_field(body, "client_id", client_id)
        if "client_secret" not in params:
            body = _append_form_field(body, "client_secret", "")
        await self.app(scope, _replay_body(body), send)


class _ScopeChallengeMiddleware:
    """Add the MCP 2025-11-25 scope hint to SDK-generated bearer challenges."""

    def __init__(self, app: ASGIApp, required_scope: str) -> None:
        self.app = app
        self.required_scope = required_scope

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        async def send_with_scope(message: Message) -> None:
            if message["type"] == "http.response.start" and message["status"] in {401, 403}:
                headers = MutableHeaders(scope=message)
                challenge = headers.get("www-authenticate")
                if challenge and "scope=" not in challenge:
                    headers["WWW-Authenticate"] = (
                        f'{challenge}, scope="{self.required_scope}"'
                    )
            await send(message)

        await self.app(scope, receive, send_with_scope)


def default_state_path() -> Path:
    """Where the OAuth state file lives.

    Deliberately NOT under the cache root: the remote deployment confines local reads to the cache,
    so a credential store placed there would be readable through the very `fetch` tool it guards.
    """
    env = os.environ.get("HARVESTER_STATE_DIR")
    base = Path(env).expanduser() if env else cache_root().parent / ".harvester-state"
    return base / "auth.json"


def _consent_page(txn: str, error: str | None = None) -> str:
    """The whole login UI. One field, because there is exactly one thing to prove."""
    banner = (
        f'<p class="err">{error}</p>' if error else
        '<p class="hint">Claude is asking to connect to your harvester server.</p>'
    )
    return f"""<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>harvester — authorize</title>
<style>
  body {{ font-family: ui-sans-serif, system-ui, sans-serif; background:#faf9f7; color:#1c1917;
         display:grid; place-items:center; min-height:100vh; margin:0; }}
  form {{ background:#fff; padding:2rem; border-radius:12px; border:1px solid #e7e5e4;
          box-shadow:0 1px 3px rgba(0,0,0,.06); width:min(92vw,26rem); }}
  h1 {{ font-size:1.1rem; margin:0 0 .25rem; }}
  .hint, .err {{ font-size:.875rem; margin:0 0 1.25rem; }}
  .hint {{ color:#57534e; }} .err {{ color:#b91c1c; }}
  input {{ width:100%; padding:.6rem .7rem; font-size:1rem; border:1px solid #d6d3d1;
           border-radius:8px; box-sizing:border-box; }}
  button {{ margin-top:1rem; width:100%; padding:.6rem; font-size:1rem; border:0;
            border-radius:8px; background:#1c1917; color:#fafaf9; cursor:pointer; }}
  @media (prefers-color-scheme: dark) {{
    body {{ background:#1c1917; color:#fafaf9; }}
    form {{ background:#292524; border-color:#44403c; }}
    .hint {{ color:#a8a29e; }}
    input {{ background:#1c1917; color:#fafaf9; border-color:#57534e; }}
    button {{ background:#fafaf9; color:#1c1917; }}
  }}
</style></head><body>
<form method="post">
  <h1>Authorize harvester</h1>
  {banner}
  <input type="hidden" name="txn" value="{txn}">
  <input type="password" name="passphrase" placeholder="Operator passphrase" autofocus required>
  <button type="submit">Authorize</button>
</form></body></html>"""


def build_app(
    public_url: str,
    *,
    user_agent: str | None = None,
    proxy_url: str | None = None,
    passphrase: str | None = None,
    static_token: str | None = None,
    state_path: Path | None = None,
    confine_local_reads: bool = True,
) -> Starlette:
    """Build the ASGI app serving harvester over Streamable HTTP at `/mcp`.

    `public_url` is the externally visible origin (e.g. https://devbox.tail46e8e9.ts.net). It must
    match what is typed into Claude, because the protected-resource metadata `resource` field is
    compared exactly.
    """
    public_url = public_url.rstrip("/")
    parsed = urlparse(public_url)
    if not parsed.hostname:
        raise ValueError(f"public_url must include a hostname, got {public_url!r}")

    if confine_local_reads:
        # Remote callers may read harvester's own artifacts and nothing else.
        detect.set_local_roots(cache_root())

    # Two independent switches. A bearer is required whenever ANY credential is configured; the
    # OAuth authorization server is mounted only when a passphrase exists to complete its consent
    # step. Advertising an authorization server whose consent screen can never succeed would send
    # Claude into a flow with no exit — an error rendered as a dead end rather than as an error.
    auth_enabled = bool(passphrase or static_token)
    oauth_enabled = bool(passphrase)
    provider = (
        HarvesterAuthProvider(
            issuer_url=public_url,
            passphrase=passphrase,
            state_path=state_path or default_state_path(),
            static_token=static_token,
            resource_url=f"{public_url}{MCP_PATH}",
        )
        if auth_enabled
        else None
    )

    # DNS-rebinding protection: ON, with the public host and loopback registered. An empty
    # allowlist here would reject every request — see module docstring.
    allowed_hosts = [parsed.netloc, parsed.hostname, f"{parsed.hostname}:*",
                     "localhost", "localhost:*", "127.0.0.1", "127.0.0.1:*"]
    security = TransportSecuritySettings(
        enable_dns_rebinding_protection=True,
        allowed_hosts=allowed_hosts,
        allowed_origins=[public_url, "http://localhost:*", "http://127.0.0.1:*"],
    )

    session_manager = StreamableHTTPSessionManager(
        app=build_server(user_agent, True, proxy_url),
        json_response=False,
        stateless=False,
        security_settings=security,
    )

    async def handle_mcp(scope, receive, send) -> None:
        await session_manager.handle_request(scope, receive, send)

    @contextlib.asynccontextmanager
    async def lifespan(_app: Starlette) -> AsyncIterator[None]:
        async with session_manager.run():
            log.info("streamable-http ready at %s%s (auth=%s, local reads=%s)",
                     public_url, MCP_PATH, "on" if auth_enabled else "OFF",
                     [str(r) for r in detect.LOCAL_ROOTS] or "unconfined")
            yield

    async def healthz(_request: Request) -> Response:
        return JSONResponse({"status": "ok", "auth": auth_enabled})

    routes: list = [Route("/healthz", healthz, methods=["GET"])]
    middleware: list[Middleware] = []

    if provider is not None:
        resource_url = AnyHttpUrl(f"{public_url}{MCP_PATH}")
        issuer = AnyHttpUrl(public_url)

        async def consent(request: Request) -> Response:
            if request.method == "GET":
                txn = request.query_params.get("txn", "")
                if not provider.pending(txn):
                    return HTMLResponse(
                        "<p>This authorization link has expired. Start again from Claude.</p>",
                        status_code=400,
                    )
                return HTMLResponse(_consent_page(txn))
            form = await request.form()
            txn = str(form.get("txn", ""))
            supplied = str(form.get("passphrase", ""))
            if not provider.pending(txn):
                return HTMLResponse(
                    "<p>This authorization link has expired. Start again from Claude.</p>",
                    status_code=400,
                )
            if not provider.check_passphrase(txn, supplied):
                return HTMLResponse(_consent_page(txn, "Incorrect passphrase."), status_code=401)
            return RedirectResponse(provider.complete_consent(txn), status_code=302)

        if oauth_enabled:
            routes.append(Route("/consent", consent, methods=["GET", "POST"]))
            registration_options = ClientRegistrationOptions(
                enabled=True, valid_scopes=[SCOPE], default_scopes=[SCOPE]
            )
            revocation_options = RevocationOptions(enabled=True)
            auth_routes = create_auth_routes(
                provider=provider,
                issuer_url=issuer,
                client_registration_options=registration_options,
                revocation_options=revocation_options,
            )
            # The SDK accepts `none` for public clients at /token and /revoke, but omits it from
            # both advertised method lists. RFC 8414 §2 requires these lists to describe what the
            # endpoints actually support; Claude also refuses to exchange a public-client code
            # unless `none` is advertised. Build the canonical document with the SDK's own builder
            # and replace only its incomplete method lists.
            metadata = build_metadata(issuer, None, registration_options, revocation_options)
            metadata.token_endpoint_auth_methods_supported = [
                "none", "client_secret_post", "client_secret_basic"
            ]
            metadata.revocation_endpoint_auth_methods_supported = [
                "none", "client_secret_post", "client_secret_basic"
            ]
            metadata_endpoint = cors_middleware(
                MetadataHandler(metadata).handle, ["GET", "OPTIONS"]
            )
            authz_metadata_index = next(
                i for i, route in enumerate(auth_routes)
                if route.path == "/.well-known/oauth-authorization-server"
            )
            auth_routes[authz_metadata_index] = Route(
                "/.well-known/oauth-authorization-server",
                endpoint=metadata_endpoint,
                methods=["GET", "OPTIONS"],
            )
            endpoint_wrappers = {
                "/authorize": lambda app: _AuthorizationResourceMiddleware(
                    app, provider, f"{public_url}{MCP_PATH}"
                ),
                "/token": lambda app: _TokenConformanceMiddleware(
                    app, f"{public_url}{MCP_PATH}"
                ),
                "/register": _RegistrationConformanceMiddleware,
                "/revoke": _RevocationConformanceMiddleware,
            }
            for i, route in enumerate(auth_routes):
                wrapper = endpoint_wrappers.get(route.path)
                if wrapper is not None:
                    auth_routes[i] = Route(
                        route.path,
                        endpoint=wrapper(
                            route.app if route.path == "/authorize" else route.endpoint
                        ),
                        methods=(
                            ["GET", "POST"] if route.path == "/authorize"
                            else ["POST", "OPTIONS"]
                        ),
                    )
            routes.extend(auth_routes)
            # RFC 8414 path-suffixed alias: some clients (incl. Claude) probe
            # /.well-known/oauth-authorization-server/mcp before the bare path. Re-register the
            # SAME route object under the alias — not a second document, the first one at a
            # second path — so the two can never drift.
            authz_metadata_route = auth_routes[authz_metadata_index]
            routes.append(
                Route(f"/.well-known/oauth-authorization-server{MCP_PATH}",
                      endpoint=authz_metadata_route.endpoint, methods=["GET", "OPTIONS"])
            )
            # OpenID Connect Discovery 1.0 alias: Anthropic's connector docs say the authorization
            # server must serve RFC 8414 metadata OR OIDC discovery. Claude probes
            # /.well-known/openid-configuration after reading the protected-resource document; a
            # 404 there ends the flow right after the user has entered their passphrase, surfacing
            # as "Couldn't register with the sign-in service" — a failure message naming a stage
            # (registration) that already succeeded. Re-register the SAME route object so this can
            # never drift from /.well-known/oauth-authorization-server.
            routes.append(
                Route("/.well-known/openid-configuration",
                      endpoint=authz_metadata_route.endpoint, methods=["GET", "OPTIONS"])
            )
            routes.append(
                Route(f"/.well-known/openid-configuration{MCP_PATH}",
                      endpoint=authz_metadata_route.endpoint, methods=["GET", "OPTIONS"])
            )

            protected_resource_routes = create_protected_resource_routes(
                resource_url=resource_url,
                authorization_servers=[issuer],
                scopes_supported=[SCOPE],
                resource_name="harvester",
            )
            routes.extend(protected_resource_routes)
            # Claude's connector probes the BARE protected-resource path in addition to the
            # path-suffixed one the SDK registers by default — see module docstring / live access
            # log (404 on the bare path stalls the flow after consent, before /token). Re-register
            # the SAME route object at the bare path so the two documents can never drift.
            routes.append(
                Route("/.well-known/oauth-protected-resource",
                      endpoint=protected_resource_routes[0].endpoint, methods=["GET", "OPTIONS"])
            )
        else:
            log.info("static token only — OAuth endpoints not mounted; clients must send the "
                     "Authorization header themselves")

        # 401 + WWW-Authenticate: Bearer resource_metadata="…" — the handshake Claude follows to
        # discover the authorization server. Claude ignores the header on a 200, so it must be a 401.
        # Without OAuth there is no metadata to point at, so the 401 carries no pointer.
        routes.append(
            Route(
                MCP_PATH,
                _ASGIEndpoint(
                    _ScopeChallengeMiddleware(
                        RequireAuthMiddleware(
                            handle_mcp,
                            [SCOPE],
                            build_resource_metadata_url(resource_url) if oauth_enabled else None,
                        ),
                        SCOPE,
                    )
                ),
                methods=MCP_METHODS,
            )
        )
        middleware.append(
            Middleware(AuthenticationMiddleware, backend=BearerAuthBackend(provider))
        )
    else:
        log.warning("auth DISABLED — no passphrase and no static token configured")
        routes.append(Route(MCP_PATH, _ASGIEndpoint(handle_mcp), methods=MCP_METHODS))

    return Starlette(routes=routes, middleware=middleware, lifespan=lifespan)


LOOPBACK_HOSTS = ("127.0.0.1", "::1", "localhost")


def _assert_bindable(host: str, authed: bool, allow_unauthenticated_bind: bool, label: str) -> None:
    """An unauthenticated gateway may only bind loopback unless explicitly allowed.

    harvester fetches arbitrary URLs on request, so an open instance on a reachable interface is an
    open proxy for whoever finds it. Loopback stays free — that is exactly the internal case.
    """
    if authed or host in LOOPBACK_HOSTS or allow_unauthenticated_bind:
        return
    raise SystemExit(
        f"refusing to bind the unauthenticated {label} gateway to {host}: harvester fetches "
        "arbitrary URLs, so an open instance is an open proxy. Bind 127.0.0.1, set "
        "HARVESTER_AUTH_PASSPHRASE / HARVESTER_STATIC_TOKEN, or pass --allow-unauthenticated if "
        "you genuinely mean it."
    )


async def _serve_gateways(gateways: "list[tuple[Starlette, str, int, str]]") -> None:
    """Run every gateway concurrently in ONE process, on one event loop.

    Each has its own uvicorn server and its own session manager, but they share this process — and
    therefore the cache, the in-flight dedup, and the negative cache in `dispatch`. One harvester,
    two doors.
    """
    import uvicorn

    servers = [
        uvicorn.Server(uvicorn.Config(app, host=host, port=port, log_level="info"))
        for app, host, port, _label in gateways
    ]
    for (_app, host, port, label), _s in zip(gateways, servers):
        log.info("%s gateway listening on %s:%d", label, host, port)
    await asyncio.gather(*(s.serve() for s in servers))


def run_http(
    public_url: str,
    host: str,
    port: int,
    *,
    internal_host: str = "127.0.0.1",
    internal_port: int = 0,
    allow_unauthenticated_bind: bool = False,
    **kwargs,
) -> None:
    """Serve over Streamable HTTP, with up to two gateways onto the SAME harvester.

    * **external** (`host:port`) — authenticated when a passphrase or static token is configured.
      This is the one a tunnel publishes.
    * **internal** (`internal_host:internal_port`, when `internal_port` is non-zero) — always
      unauthenticated, loopback by default. For local/tailnet use where typing a bearer token onto
      every call is friction with no security benefit, because the port is not reachable anyway.

    The split is enforced by the NETWORK, not by a path: the internal gateway is a separate socket,
    so publishing the external port cannot accidentally expose the open one. Two doors, one house —
    never two paths on the same door.
    """
    authed = bool(kwargs.get("passphrase") or kwargs.get("static_token"))
    _assert_bindable(host, authed, allow_unauthenticated_bind, "external")

    gateways: "list[tuple[Starlette, str, int, str]]" = [
        (build_app(public_url, **kwargs), host, port, "external")
    ]

    if internal_port:
        if not authed:
            # The external gateway is already open; a second open door adds risk and no capability.
            log.warning("internal gateway skipped — no credentials configured, so the external "
                        "gateway on %s:%d is already unauthenticated", host, port)
        else:
            _assert_bindable(internal_host, False, allow_unauthenticated_bind, "internal")
            open_kwargs = {k: v for k, v in kwargs.items()
                           if k not in ("passphrase", "static_token")}
            gateways.append((
                build_app(f"http://{internal_host}:{internal_port}", **open_kwargs),
                internal_host, internal_port, "internal",
            ))

    asyncio.run(_serve_gateways(gateways))
