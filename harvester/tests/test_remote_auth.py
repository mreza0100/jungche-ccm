"""Tests for the remote (Streamable HTTP) transport, its OAuth 2.1 server, and local-read confinement.

These pin the failure modes that are SILENT in production — a connector that simply "can't reach the
server" with no useful error, or a confinement that quietly reads more than it should.
"""

import base64
import hashlib
import secrets
import urllib.parse
from pathlib import Path

import pytest
from starlette.testclient import TestClient

from harvester import detect, remote
from harvester.auth import SCOPE, HarvesterAuthProvider
from mcp.server.auth.provider import AccessToken

PASSPHRASE = "correct-horse-battery-staple"
STATIC = "static-token-for-tests"
PUBLIC = "https://harvester.example.ts.net"
REDIRECT = "https://claude.ai/api/mcp/auth_callback"
MCP_HEADERS = {
    "Content-Type": "application/json",
    "Accept": "application/json, text/event-stream",
}
INIT = {
    "jsonrpc": "2.0", "id": 1, "method": "initialize",
    "params": {"protocolVersion": "2025-06-18", "capabilities": {},
               "clientInfo": {"name": "t", "version": "1"}},
}


@pytest.fixture(autouse=True)
def restore_local_roots():
    """`build_app` mutates the module-level confinement; put it back so tests stay independent."""
    original = detect.LOCAL_ROOTS
    yield
    detect.LOCAL_ROOTS = original


@pytest.fixture
def authed_app(tmp_path):
    return remote.build_app(
        PUBLIC, passphrase=PASSPHRASE, static_token=STATIC,
        state_path=tmp_path / "auth.json",
    )


@pytest.fixture
def open_app(tmp_path):
    """No passphrase, no static token — the local/internal deployment."""
    return remote.build_app(PUBLIC, state_path=tmp_path / "auth.json")


# ── transport wiring ─────────────────────────────────────────────────────────


def test_mcp_served_at_exact_path_without_redirect(authed_app):
    """Regression: `Mount("/mcp")` answers /mcp with a 307 to /mcp/. Claude compares the
    protected-resource `resource` field against the URL the user typed, so a redirect here breaks
    the connector in a way that only shows up as "couldn't reach the MCP server"."""
    with TestClient(authed_app, base_url=PUBLIC) as c:
        r = c.post("/mcp", headers=MCP_HEADERS, json=INIT, follow_redirects=False)
        assert r.status_code != 307, "endpoint must be served AT /mcp, not redirected"
        assert r.status_code == 401  # unauthenticated, but reached the endpoint


def test_public_host_is_allowed_by_dns_rebinding_protection(open_app):
    """The SDK enables DNS-rebinding protection with an EMPTY allowlist, which rejects everything.
    The public host must be registered or every request 4xxs with no clue why."""
    with TestClient(open_app, base_url=PUBLIC) as c:
        r = c.post("/mcp", headers=MCP_HEADERS, json=INIT)
        assert r.status_code == 200
        assert "harvester" in r.text


def test_foreign_host_header_is_rejected(open_app):
    with TestClient(open_app, base_url=PUBLIC) as c:
        r = c.post("/mcp", headers={**MCP_HEADERS, "Host": "evil.example.com"}, json=INIT)
        assert r.status_code >= 400


def test_public_url_without_hostname_is_refused(tmp_path):
    with pytest.raises(ValueError, match="hostname"):
        remote.build_app("not-a-url", state_path=tmp_path / "auth.json")


# ── unauthenticated (local/internal) mode ────────────────────────────────────


def test_open_app_serves_mcp_without_credentials(open_app):
    with TestClient(open_app, base_url=PUBLIC) as c:
        assert c.post("/mcp", headers=MCP_HEADERS, json=INIT).status_code == 200
        assert c.get("/healthz").json() == {"status": "ok", "auth": False}


def test_open_app_publishes_no_oauth_metadata(open_app):
    """An unauthenticated server must not advertise an authorization server it doesn't run."""
    with TestClient(open_app, base_url=PUBLIC) as c:
        oauth_paths = [
            "/.well-known/oauth-protected-resource/mcp",
            "/.well-known/oauth-protected-resource",
            "/.well-known/oauth-authorization-server",
            "/.well-known/oauth-authorization-server/mcp",
            "/.well-known/openid-configuration",
            "/.well-known/openid-configuration/mcp",
            "/authorize",
            "/token",
            "/register",
            "/revoke",
            "/consent",
        ]
        for path in oauth_paths:
            assert c.get(path).status_code == 404, path
            assert c.post(path).status_code == 404, path


def test_unauthenticated_public_bind_is_refused():
    """An open harvester on a public interface is an open fetch proxy — that needs to be a choice."""
    with pytest.raises(SystemExit, match="unauthenticated"):
        remote.run_http(PUBLIC, "0.0.0.0", 8081, internal_port=0)


@pytest.fixture
def captured_gateways(monkeypatch):
    """Capture what run_http would bind, without binding anything."""
    seen: list = []

    async def fake_serve(gateways):
        seen.extend((host, port, label) for _app, host, port, label in gateways)

    monkeypatch.setattr(remote, "_serve_gateways", fake_serve)
    return seen


def test_unauthenticated_loopback_bind_is_allowed(captured_gateways, tmp_path):
    """...but loopback is exactly the internal case, so it must not be blocked."""
    remote.run_http("http://127.0.0.1:8081", "127.0.0.1", 8081,
                    internal_port=0, state_path=tmp_path / "auth.json")
    assert captured_gateways == [("127.0.0.1", 8081, "external")]


# ── dual gateway: one instance, two doors ────────────────────────────────────


def test_two_gateways_run_from_one_instance(captured_gateways, tmp_path):
    remote.run_http(PUBLIC, "0.0.0.0", 8081,
                    internal_host="127.0.0.1", internal_port=8082,
                    passphrase=PASSPHRASE, state_path=tmp_path / "auth.json")
    assert captured_gateways == [
        ("0.0.0.0", 8081, "external"),
        ("127.0.0.1", 8082, "internal"),
    ]


def test_internal_gateway_needs_no_credentials(tmp_path):
    """The internal door is open by design — that is the whole point of it."""
    app = remote.build_app("http://127.0.0.1:8082", state_path=tmp_path / "auth.json")
    with TestClient(app, base_url="http://127.0.0.1:8082") as c:
        assert c.post("/mcp", headers=MCP_HEADERS, json=INIT).status_code == 200


def test_internal_gateway_skipped_when_external_is_already_open(captured_gateways, tmp_path):
    """With no credentials the external gateway is itself unauthenticated; a second open door
    would add exposure and no capability."""
    remote.run_http("http://127.0.0.1:8081", "127.0.0.1", 8081,
                    internal_port=8082, state_path=tmp_path / "auth.json")
    assert captured_gateways == [("127.0.0.1", 8081, "external")]


def test_internal_gateway_refuses_a_public_bind(tmp_path):
    """Publishing the external port must never be able to expose the open door with it."""
    with pytest.raises(SystemExit, match="internal"):
        remote.run_http(PUBLIC, "0.0.0.0", 8081,
                        internal_host="0.0.0.0", internal_port=8082,
                        passphrase=PASSPHRASE, state_path=tmp_path / "auth.json")


# ── the 401 handshake and discovery documents ────────────────────────────────


def test_401_carries_resource_metadata_pointer(authed_app):
    """Claude discovers the authorization server from this header. Without it the connection dies
    with no diagnostic; Claude also ignores the header on a 200, so the status must be 401."""
    with TestClient(authed_app, base_url=PUBLIC) as c:
        r = c.post("/mcp", headers=MCP_HEADERS, json=INIT)
        assert r.status_code == 401
        www = r.headers["www-authenticate"]
        assert 'resource_metadata="' in www
        assert f"{PUBLIC}/.well-known/oauth-protected-resource/mcp" in www
        assert 'scope="harvest"' in www


def test_protected_resource_metadata_matches_entered_url(authed_app):
    """`resource` is compared EXACTLY against the URL typed into Claude."""
    with TestClient(authed_app, base_url=PUBLIC) as c:
        meta = c.get("/.well-known/oauth-protected-resource/mcp").json()
        assert meta["resource"] == f"{PUBLIC}/mcp"
        assert meta["authorization_servers"] == [f"{PUBLIC}/"]


def test_bare_protected_resource_metadata_is_served(authed_app):
    """Regression: Claude's connector probes the BARE
    /.well-known/oauth-protected-resource in addition to the path-suffixed one. Live prod
    access log for the failing flow:

        GET /.well-known/oauth-protected-resource -> 404
        POST /mcp -> 401
        GET /.well-known/oauth-protected-resource -> 404

    The 404 on the bare path breaks the handshake: Claude completes consent, then never calls
    /token. The user sees "Couldn't register with the sign-in service" right after successfully
    entering the passphrase — a failure message pointing at the wrong stage entirely."""
    with TestClient(authed_app, base_url=PUBLIC) as c:
        meta = c.get("/.well-known/oauth-protected-resource").json()
        assert meta["resource"] == f"{PUBLIC}/mcp"


def test_bare_and_suffixed_protected_resource_metadata_cannot_drift(authed_app):
    """Anti-drift pin: both paths must serve the byte-for-byte identical document, not two
    independently-maintained copies that can fall out of sync."""
    with TestClient(authed_app, base_url=PUBLIC) as c:
        bare = c.get("/.well-known/oauth-protected-resource")
        suffixed = c.get("/.well-known/oauth-protected-resource/mcp")
        assert bare.content == suffixed.content


def test_authorization_server_metadata_advertises_s256_and_dcr(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        meta = c.get("/.well-known/oauth-authorization-server").json()
        assert meta["code_challenge_methods_supported"] == ["S256"]
        assert meta["registration_endpoint"] == f"{PUBLIC}/register"
        assert meta["scopes_supported"] == ["harvest"]
        assert "offline_access" not in meta["scopes_supported"]


def test_authorization_server_metadata_advertises_every_accepted_client_auth_method(authed_app):
    """DCR registers Claude with `none`; omitting it makes Claude abandon the flow before /token."""
    with TestClient(authed_app, base_url=PUBLIC) as c:
        meta = c.get("/.well-known/oauth-authorization-server").json()
        expected = ["none", "client_secret_post", "client_secret_basic"]
        assert meta["token_endpoint_auth_methods_supported"] == expected
        assert meta["revocation_endpoint_auth_methods_supported"] == expected

        client_id = _register(c)
        verifier, challenge = _pkce()
        txn = _authorize(c, client_id, challenge)
        code = urllib.parse.parse_qs(
            urllib.parse.urlparse(_consent(c, txn).headers["location"]).query
        )["code"][0]
        token = c.post("/token", data={
            "grant_type": "authorization_code", "code": code, "redirect_uri": REDIRECT,
            "client_id": client_id, "code_verifier": verifier,
        })
        assert token.status_code == 200


def test_suffixed_authorization_server_metadata_matches_root(authed_app):
    """RFC 8414 path-suffixed alias, for the same both-locations robustness as the
    protected-resource document above."""
    with TestClient(authed_app, base_url=PUBLIC) as c:
        root = c.get("/.well-known/oauth-authorization-server")
        suffixed = c.get("/.well-known/oauth-authorization-server/mcp")
        assert suffixed.content == root.content


def test_openid_configuration_matches_authorization_server_metadata(authed_app):
    """Regression: live prod access log for the failing flow:

        GET /.well-known/oauth-protected-resource -> 200
        GET /.well-known/openid-configuration -> 404

    Anthropic's connector docs require RFC 8414 authorization server metadata OR OpenID Connect
    Discovery 1.0; we only served the former. Claude reads the protected-resource document, then
    probes OIDC discovery and abandons the flow on the 404 — right after the user has entered
    their passphrase, so the visible failure ("Couldn't register with the sign-in service") names
    a stage that already succeeded."""
    with TestClient(authed_app, base_url=PUBLIC) as c:
        authz = c.get("/.well-known/oauth-authorization-server")
        oidc = c.get("/.well-known/openid-configuration")
        assert oidc.status_code == 200
        assert oidc.content == authz.content


def test_openid_configuration_suffixed_matches_authorization_server_metadata(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        authz = c.get("/.well-known/oauth-authorization-server")
        oidc = c.get("/.well-known/openid-configuration/mcp")
        assert oidc.status_code == 200
        assert oidc.content == authz.content


def test_every_metadata_alias_is_json_and_cors_readable(authed_app):
    paths = [
        "/.well-known/oauth-protected-resource/mcp",
        "/.well-known/oauth-protected-resource",
        "/.well-known/oauth-authorization-server",
        "/.well-known/oauth-authorization-server/mcp",
        "/.well-known/openid-configuration",
        "/.well-known/openid-configuration/mcp",
    ]
    with TestClient(authed_app, base_url=PUBLIC) as c:
        for path in paths:
            response = c.get(path, headers={"Origin": "https://client.example"})
            assert response.status_code == 200, path
            assert response.headers["content-type"] == "application/json", path
            assert response.headers["access-control-allow-origin"] == "*", path


# ── bearer tokens ────────────────────────────────────────────────────────────


def test_static_token_is_accepted(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        r = c.post("/mcp", headers={**MCP_HEADERS, "Authorization": f"Bearer {STATIC}"}, json=INIT)
        assert r.status_code == 200


@pytest.mark.parametrize("bad", ["wrong-token", STATIC + "x", STATIC[:-1], ""])
def test_wrong_token_is_rejected(authed_app, bad):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        r = c.post("/mcp", headers={**MCP_HEADERS, "Authorization": f"Bearer {bad}"}, json=INIT)
        assert r.status_code == 401


def test_static_token_only_requires_bearer_but_mounts_no_oauth(tmp_path):
    """With no passphrase there is no way to complete a consent screen, so advertising an
    authorization server would send Claude into a flow with no exit."""
    app = remote.build_app(PUBLIC, static_token=STATIC, state_path=tmp_path / "auth.json")
    with TestClient(app, base_url=PUBLIC) as c:
        assert c.get("/.well-known/oauth-authorization-server").status_code == 404
        assert c.get("/consent").status_code == 404
        assert c.post("/mcp", headers=MCP_HEADERS, json=INIT).status_code == 401
        r = c.post("/mcp", headers={**MCP_HEADERS, "Authorization": f"Bearer {STATIC}"}, json=INIT)
        assert r.status_code == 200


def test_corrupt_state_file_does_not_crash_startup(tmp_path):
    """A half-written state file must degrade to "re-authorize", never to a server that won't boot."""
    state = tmp_path / "auth.json"
    state.write_text('{"clients": [], "refresh": [{"no_hash_key": true}]}', encoding="utf-8")
    provider = HarvesterAuthProvider(PUBLIC, PASSPHRASE, state, None)
    assert provider._refresh == {}


def test_static_token_absent_when_not_configured(tmp_path):
    """A server configured with only a passphrase must not accept an empty/any static token."""
    app = remote.build_app(PUBLIC, passphrase=PASSPHRASE, state_path=tmp_path / "auth.json")
    with TestClient(app, base_url=PUBLIC) as c:
        r = c.post("/mcp", headers={**MCP_HEADERS, "Authorization": "Bearer "}, json=INIT)
        assert r.status_code == 401


# ── the OAuth flow ───────────────────────────────────────────────────────────


def _pkce():
    verifier = secrets.token_urlsafe(64)
    challenge = base64.urlsafe_b64encode(
        hashlib.sha256(verifier.encode()).digest()
    ).decode().rstrip("=")
    return verifier, challenge


def _register(c, *, auth_method="none", redirect_uri=REDIRECT):
    r = c.post("/register", json={
        "client_name": "test", "redirect_uris": [redirect_uri],
        "grant_types": ["authorization_code", "refresh_token"],
        "response_types": ["code"], "token_endpoint_auth_method": auth_method, "scope": "harvest",
    })
    assert r.status_code in (200, 201), r.text
    return r.json()["client_id"]


def _authorize(c, client_id, challenge, *, resource=None):
    params = {
        "response_type": "code", "client_id": client_id, "redirect_uri": REDIRECT,
        "code_challenge": challenge, "code_challenge_method": "S256",
        "state": "st4te", "scope": "harvest",
    }
    if resource is not None:
        params["resource"] = resource
    r = c.get("/authorize", params=params, follow_redirects=False)
    loc = r.headers["location"]
    return urllib.parse.parse_qs(urllib.parse.urlparse(loc).query)["txn"][0]


def _consent(c, txn, passphrase=PASSPHRASE):
    return c.post("/consent", data={"txn": txn, "passphrase": passphrase},
                  follow_redirects=False)


def test_registration_requires_json_and_disables_response_caching(authed_app):
    payload = {
        "redirect_uris": [REDIRECT],
        "grant_types": ["authorization_code", "refresh_token"],
        "response_types": ["code"],
        "token_endpoint_auth_method": "none",
        "scope": "harvest",
    }
    with TestClient(authed_app, base_url=PUBLIC) as c:
        response = c.post("/register", json=payload)
        assert response.status_code == 201
        assert response.headers["content-type"] == "application/json"
        assert response.headers["cache-control"] == "no-store"
        assert response.headers["pragma"] == "no-cache"

        wrong_type = c.post("/register", data=payload)
        assert wrong_type.status_code == 400
        assert wrong_type.json()["error"] == "invalid_client_metadata"

        malformed = c.post(
            "/register", content="{", headers={"Content-Type": "application/json"}
        )
        assert malformed.status_code == 400
        assert malformed.json()["error"] == "invalid_client_metadata"


def test_registration_uses_rfc7591_default_and_complete_secret_metadata(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        response = c.post("/register", json={
            "redirect_uris": [REDIRECT],
            "grant_types": ["authorization_code", "refresh_token"],
            "response_types": ["code"],
            "scope": "harvest",
        })
        assert response.status_code == 201
        registered = response.json()
        assert registered["token_endpoint_auth_method"] == "client_secret_basic"
        assert registered["client_secret"]
        assert registered["client_secret_expires_at"] == 0


@pytest.mark.parametrize(
    "auth_method, redirect_uri, error",
    [
        ("private_key_jwt", REDIRECT, "invalid_client_metadata"),
        ("none", "http://client.example/callback", "invalid_redirect_uri"),
        ("none", "https://client.example/callback#fragment", "invalid_redirect_uri"),
    ],
)
def test_registration_rejects_metadata_the_server_cannot_honor(
    authed_app, auth_method, redirect_uri, error
):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        response = c.post("/register", json={
            "redirect_uris": [redirect_uri],
            "grant_types": ["authorization_code", "refresh_token"],
            "response_types": ["code"],
            "token_endpoint_auth_method": auth_method,
            "scope": "harvest",
        })
        assert response.status_code == 400
        assert response.json()["error"] == error


def test_http_loopback_redirect_is_allowed_for_native_clients(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c, redirect_uri="http://127.0.0.1:43123/callback")
        assert client_id


def test_full_authorization_code_flow(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        verifier, challenge = _pkce()
        txn = _authorize(c, client_id, challenge)
        r = _consent(c, txn)
        assert r.status_code == 302
        q = urllib.parse.parse_qs(urllib.parse.urlparse(r.headers["location"]).query)
        assert q["state"] == ["st4te"]
        code = q["code"][0]

        r = c.post("/token", data={
            "grant_type": "authorization_code", "code": code, "redirect_uri": REDIRECT,
            "client_id": client_id, "code_verifier": verifier,
        })
        assert r.status_code == 200, r.text
        access = r.json()["access_token"]

        r = c.post("/mcp", headers={**MCP_HEADERS, "Authorization": f"Bearer {access}"}, json=INIT)
        assert r.status_code == 200


def test_resource_indicator_is_validated_on_authorize_and_token(authed_app):
    """MCP 2025-06-18+ sends the canonical MCP URI at both authorization-server endpoints."""
    uppercase_resource = "HTTPS://HARVESTER.EXAMPLE.TS.NET/mcp"
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        verifier, challenge = _pkce()
        txn = _authorize(c, client_id, challenge, resource=uppercase_resource)
        code = urllib.parse.parse_qs(
            urllib.parse.urlparse(_consent(c, txn).headers["location"]).query
        )["code"][0]
        token = c.post("/token", data={
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": REDIRECT,
            "client_id": client_id,
            "code_verifier": verifier,
            "resource": uppercase_resource,
        })
        assert token.status_code == 200, token.text
        access = token.json()["access_token"]
        assert c.post(
            "/mcp",
            headers={**MCP_HEADERS, "Authorization": f"Bearer {access}"},
            json=INIT,
        ).status_code == 200


def test_authorize_rejects_a_resource_for_another_server(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        _, challenge = _pkce()
        response = c.get("/authorize", params={
            "response_type": "code",
            "client_id": client_id,
            "redirect_uri": REDIRECT,
            "code_challenge": challenge,
            "code_challenge_method": "S256",
            "state": "st4te",
            "scope": "harvest",
            "resource": "https://other.example/mcp",
        }, follow_redirects=False)
        assert response.status_code == 302
        error = urllib.parse.parse_qs(
            urllib.parse.urlparse(response.headers["location"]).query
        )
        assert error["error"] == ["invalid_target"]
        assert error["state"] == ["st4te"]


def test_token_rejects_a_resource_for_another_server_without_burning_code(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        verifier, challenge = _pkce()
        txn = _authorize(c, client_id, challenge, resource=f"{PUBLIC}/mcp")
        code = urllib.parse.parse_qs(
            urllib.parse.urlparse(_consent(c, txn).headers["location"]).query
        )["code"][0]
        body = {
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": REDIRECT,
            "client_id": client_id,
            "code_verifier": verifier,
        }
        wrong = c.post("/token", data={**body, "resource": "https://other.example/mcp"})
        assert wrong.status_code == 400
        assert wrong.json()["error"] == "invalid_target"
        assert c.post("/token", data={**body, "resource": f"{PUBLIC}/mcp"}).status_code == 200


def test_confidential_client_can_use_standard_http_basic_token_auth(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        registration = c.post("/register", json={
            "redirect_uris": [REDIRECT],
            "grant_types": ["authorization_code", "refresh_token"],
            "response_types": ["code"],
            "token_endpoint_auth_method": "client_secret_basic",
            "scope": "harvest",
        }).json()
        verifier, challenge = _pkce()
        txn = _authorize(c, registration["client_id"], challenge)
        code = urllib.parse.parse_qs(
            urllib.parse.urlparse(_consent(c, txn).headers["location"]).query
        )["code"][0]
        credentials = base64.b64encode(
            f'{registration["client_id"]}:{registration["client_secret"]}'.encode()
        ).decode()
        response = c.post("/token", headers={"Authorization": f"Basic {credentials}"}, data={
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": REDIRECT,
            "code_verifier": verifier,
        })
        assert response.status_code == 200, response.text


def test_token_uses_rfc6749_error_for_unsupported_grant(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        response = c.post("/token", data={
            "grant_type": "client_credentials",
            "client_id": client_id,
        })
        assert response.status_code == 400
        assert response.json()["error"] == "unsupported_grant_type"


def test_token_and_revocation_endpoints_require_form_encoding(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        token = c.post("/token", json={
            "grant_type": "refresh_token",
            "refresh_token": "unused",
            "client_id": client_id,
        })
        assert token.status_code == 400
        assert token.json()["error"] == "invalid_request"

        revocation = c.post("/revoke", json={"token": "unused", "client_id": client_id})
        assert revocation.status_code == 400
        assert revocation.json()["error"] == "invalid_request"


def test_pkce_rejects_a_wrong_verifier(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        _, challenge = _pkce()
        txn = _authorize(c, client_id, challenge)
        code = urllib.parse.parse_qs(
            urllib.parse.urlparse(_consent(c, txn).headers["location"]).query
        )["code"][0]
        r = c.post("/token", data={
            "grant_type": "authorization_code", "code": code, "redirect_uri": REDIRECT,
            "client_id": client_id, "code_verifier": secrets.token_urlsafe(64),
        })
        assert r.status_code == 400
        assert r.json()["error"] == "invalid_grant"


def test_pkce_rejects_a_malformed_challenge(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        response = c.get("/authorize", params={
            "response_type": "code",
            "client_id": client_id,
            "redirect_uri": REDIRECT,
            "code_challenge": "too-short",
            "code_challenge_method": "S256",
            "state": "st4te",
            "scope": "harvest",
        }, follow_redirects=False)
        error = urllib.parse.parse_qs(
            urllib.parse.urlparse(response.headers["location"]).query
        )
        assert error["error"] == ["invalid_request"]


def test_wrong_passphrase_mints_no_code(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        _, challenge = _pkce()
        txn = _authorize(c, client_id, challenge)
        r = _consent(c, txn, "not-the-passphrase")
        assert r.status_code == 401
        assert "location" not in r.headers


def test_consent_transaction_burns_after_repeated_failures(authed_app):
    """A parked /authorize URL must not be a brute-force oracle."""
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        _, challenge = _pkce()
        txn = _authorize(c, client_id, challenge)
        for _ in range(6):
            _consent(c, txn, "wrong")
        # Even the CORRECT passphrase must now fail — the transaction is gone.
        r = _consent(c, txn)
        assert r.status_code == 400


def test_authorization_code_is_single_use(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        verifier, challenge = _pkce()
        txn = _authorize(c, client_id, challenge)
        code = urllib.parse.parse_qs(
            urllib.parse.urlparse(_consent(c, txn).headers["location"]).query
        )["code"][0]
        body = {"grant_type": "authorization_code", "code": code, "redirect_uri": REDIRECT,
                "client_id": client_id, "code_verifier": verifier}
        assert c.post("/token", data=body).status_code == 200
        replay = c.post("/token", data=body)
        assert replay.status_code == 400
        assert replay.json()["error"] == "invalid_grant"


def test_refresh_token_rotates_and_old_one_dies(authed_app):
    """OAuth 2.1 requires rotation for public clients, and DCR registers Claude as one."""
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        verifier, challenge = _pkce()
        txn = _authorize(c, client_id, challenge)
        code = urllib.parse.parse_qs(
            urllib.parse.urlparse(_consent(c, txn).headers["location"]).query
        )["code"][0]
        first = c.post("/token", data={
            "grant_type": "authorization_code", "code": code, "redirect_uri": REDIRECT,
            "client_id": client_id, "code_verifier": verifier,
        }).json()

        r = c.post("/token", data={"grant_type": "refresh_token",
                                   "refresh_token": first["refresh_token"],
                                   "client_id": client_id})
        assert r.status_code == 200
        assert r.json()["refresh_token"] != first["refresh_token"]
        refreshed_access = r.json()["access_token"]
        assert c.post(
            "/mcp",
            headers={**MCP_HEADERS, "Authorization": f"Bearer {refreshed_access}"},
            json=INIT,
        ).status_code == 200

        replay = c.post("/token", data={"grant_type": "refresh_token",
                                        "refresh_token": first["refresh_token"],
                                        "client_id": client_id})
        assert replay.status_code == 400
        assert replay.json()["error"] == "invalid_grant"


def test_refresh_rejects_a_resource_for_another_server(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        verifier, challenge = _pkce()
        txn = _authorize(c, client_id, challenge)
        code = urllib.parse.parse_qs(
            urllib.parse.urlparse(_consent(c, txn).headers["location"]).query
        )["code"][0]
        first = c.post("/token", data={
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": REDIRECT,
            "client_id": client_id,
            "code_verifier": verifier,
        }).json()
        response = c.post("/token", data={
            "grant_type": "refresh_token",
            "refresh_token": first["refresh_token"],
            "client_id": client_id,
            "resource": "https://other.example/mcp",
        })
        assert response.status_code == 400
        assert response.json()["error"] == "invalid_target"


def test_public_client_can_revoke_without_a_client_secret(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        verifier, challenge = _pkce()
        txn = _authorize(c, client_id, challenge)
        code = urllib.parse.parse_qs(
            urllib.parse.urlparse(_consent(c, txn).headers["location"]).query
        )["code"][0]
        token = c.post("/token", data={
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": REDIRECT,
            "client_id": client_id,
            "code_verifier": verifier,
        }).json()
        revoked = c.post("/revoke", data={
            "token": token["access_token"],
            "token_type_hint": "access_token",
            "client_id": client_id,
        })
        assert revoked.status_code == 200
        assert revoked.headers["cache-control"] == "no-store"
        assert revoked.headers["pragma"] == "no-cache"
        assert c.post(
            "/mcp",
            headers={**MCP_HEADERS, "Authorization": f'Bearer {token["access_token"]}'},
            json=INIT,
        ).status_code == 401


def test_revocation_of_an_unknown_token_is_idempotently_successful(authed_app):
    with TestClient(authed_app, base_url=PUBLIC) as c:
        client_id = _register(c)
        response = c.post("/revoke", data={
            "token": "does-not-exist",
            "client_id": client_id,
        })
        assert response.status_code == 200


async def test_resource_server_rejects_an_access_token_for_another_audience(tmp_path):
    provider = HarvesterAuthProvider(
        PUBLIC,
        PASSPHRASE,
        tmp_path / "auth.json",
        resource_url=f"{PUBLIC}/mcp",
    )
    raw_token = "wrong-audience-token"
    provider._access[hashlib.sha256(raw_token.encode()).hexdigest()] = AccessToken(
        token=raw_token,
        client_id="test",
        scopes=[SCOPE],
        resource="https://other.example/mcp",
    )
    assert await provider.verify_token(raw_token) is None


# ── credential storage ───────────────────────────────────────────────────────


def test_state_file_lives_outside_the_cache_root(tmp_path, monkeypatch):
    """The remote deployment makes the cache root readable via `fetch`. A credential store inside
    it would be readable through the very tool it protects."""
    from harvester.cache import cache_root
    assert not str(remote.default_state_path()).startswith(str(cache_root()) + "/")


def test_state_file_stores_no_live_tokens(tmp_path):
    """A leaked state file must be a list of useless digests, not working credentials."""
    state = tmp_path / "auth.json"
    app = remote.build_app(PUBLIC, passphrase=PASSPHRASE, state_path=state)
    with TestClient(app, base_url=PUBLIC) as c:
        client_id = _register(c)
        verifier, challenge = _pkce()
        txn = _authorize(c, client_id, challenge)
        code = urllib.parse.parse_qs(
            urllib.parse.urlparse(_consent(c, txn).headers["location"]).query
        )["code"][0]
        tok = c.post("/token", data={
            "grant_type": "authorization_code", "code": code, "redirect_uri": REDIRECT,
            "client_id": client_id, "code_verifier": verifier,
        }).json()

    raw = state.read_text(encoding="utf-8")
    assert tok["access_token"] not in raw
    assert tok["refresh_token"] not in raw


def test_state_file_is_owner_only(tmp_path):
    state = tmp_path / "auth.json"
    provider = HarvesterAuthProvider(PUBLIC, PASSPHRASE, state, None)
    provider._save()
    assert state.stat().st_mode & 0o077 == 0, "auth state must not be group/world readable"


# ── local-read confinement ───────────────────────────────────────────────────


def test_confinement_allows_the_cache_and_refuses_everything_else(tmp_path):
    cache = tmp_path / ".cache"
    cache.mkdir(exist_ok=True)
    inside = cache / "artifact.md"
    inside.write_text("cached", encoding="utf-8")
    outside = tmp_path / "secrets.md"
    outside.write_text("private", encoding="utf-8")

    detect.set_local_roots(cache)
    assert detect.deny_reason(inside) is None
    assert detect.deny_reason(outside) is not None
    assert detect.deny_reason(Path("/etc/hostname")) is not None


def test_confinement_follows_symlinks_out(tmp_path):
    """The cache dir is writable by the fetch path — a planted symlink must not become an escape."""
    cache = tmp_path / ".cache"
    cache.mkdir(exist_ok=True)
    secret = tmp_path / "secret.md"
    secret.write_text("private", encoding="utf-8")
    link = cache / "innocent.md"
    link.symlink_to(secret)

    detect.set_local_roots(cache)
    assert detect.deny_reason(link) is not None


def test_confinement_blocks_parent_traversal(tmp_path):
    cache = tmp_path / ".cache"
    cache.mkdir(exist_ok=True)
    (tmp_path / "secret.md").write_text("private", encoding="utf-8")

    detect.set_local_roots(cache)
    assert detect.deny_reason(cache / ".." / "secret.md") is not None


def test_denylist_still_applies_inside_the_roots(tmp_path):
    """Confinement replaces nothing — a credential file inside the cache is still refused."""
    cache = tmp_path / ".cache"
    cache.mkdir(exist_ok=True)
    key = cache / "leaked.pem"
    key.write_text("-----BEGIN PRIVATE KEY-----", encoding="utf-8")

    detect.set_local_roots(cache)
    assert detect.deny_reason(key) is not None


def test_unconfined_by_default(tmp_path):
    """The stdio server's caller already owns the machine; confinement is a remote-only posture."""
    doc = tmp_path / "notes.md"
    doc.write_text("hello", encoding="utf-8")
    detect.set_local_roots()
    assert detect.deny_reason(doc) is None


def test_build_app_confines_reads_to_the_cache(tmp_path):
    from harvester.cache import cache_root
    remote.build_app(PUBLIC, state_path=tmp_path / "auth.json")
    assert detect.LOCAL_ROOTS == (cache_root().resolve(),)


def test_confinement_can_be_disabled_explicitly(tmp_path):
    remote.build_app(PUBLIC, state_path=tmp_path / "auth.json", confine_local_reads=False)
    assert detect.LOCAL_ROOTS == ()
