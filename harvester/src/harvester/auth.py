"""Single-operator OAuth 2.1 authorization server for the remote (Streamable HTTP) deployment.

harvester has no user model and never will — it is one person's fetch tool. But claude.ai speaks
OAuth 2.1 and nothing else on the plans that matter (its static-header mode is a gated beta), so
this module is the smallest honest authorization server that satisfies the MCP authorization spec:
DCR, PKCE S256, refresh rotation, revocation. "Signing in" means proving you know the operator
passphrase.

What this module does NOT implement, on purpose:
  * PKCE verifier and redirect_uri consistency checks — the SDK's `TokenHandler` owns both.
  * Client authentication — the SDK owns `none`, `client_secret_post`, and `client_secret_basic`;
    this provider constrains DCR to exactly that advertised set.

Storage note: the state file lives OUTSIDE the cache root by construction. The remote deployment
confines local reads to the cache (`detect.LOCAL_ROOTS`), so anything under it is fetchable by a
remote caller — a credential store there would be readable through the very tool it protects.
"""

import hashlib
import ipaddress
import json
import os
import re
import secrets
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit

from mcp.server.auth.provider import (
    AccessToken,
    AuthorizationCode,
    AuthorizationParams,
    AuthorizeError,
    OAuthAuthorizationServerProvider,
    RegistrationError,
    RefreshToken,
    construct_redirect_uri,
)
from mcp.shared.auth import OAuthClientInformationFull, OAuthToken

from .log import get_logger

log = get_logger("auth")

SCOPE = "harvest"
"""The single scope. harvester's tools are all-or-nothing — a partial grant would be a lie."""

AUTH_CODE_TTL = 300          # 5 min, per OAuth 2.1 guidance for authorization codes
ACCESS_TOKEN_TTL = 3600      # 1 h — Claude refreshes reactively on 401 and proactively before it
CONSENT_TXN_TTL = 600        # 10 min for the operator to type the passphrase
MAX_CONSENT_ATTEMPTS = 5     # per transaction, then it is burned
_PKCE_CHALLENGE = re.compile(r"^[A-Za-z0-9._~-]{43,128}$")


def _hash(token: str) -> str:
    """sha256 of a bearer token, so no live credential is ever written to disk.

    Pinned by `tests/test_remote_auth.py::test_state_file_stores_no_live_tokens`, which asserts the
    issued access and refresh tokens do not appear in the state file. That test exists because the
    first cut of this module persisted `RefreshToken.token` verbatim while this docstring already
    claimed otherwise — the claim is only worth what the pin proves.
    """
    return hashlib.sha256(token.encode()).hexdigest()


def _now() -> int:
    return int(time.time())


def resource_matches(candidate: str, expected: str) -> bool:
    """Compare MCP resource URIs, accepting only scheme/host case differences.

    MCP 2025-06-18 and later define the resource as a canonical absolute URI and recommend
    accepting uppercase scheme/host for interoperability. Path, query, explicit port, userinfo,
    and trailing-slash differences remain significant.
    """
    try:
        candidate_url = urlsplit(candidate)
        expected_url = urlsplit(expected)
        if (
            not candidate_url.scheme
            or not candidate_url.hostname
            or candidate_url.fragment
            or expected_url.fragment
        ):
            return False
        candidate_identity = (
            candidate_url.scheme.lower(),
            candidate_url.username,
            candidate_url.password,
            candidate_url.hostname.lower(),
            candidate_url.port,
            candidate_url.path,
            candidate_url.query,
        )
        expected_identity = (
            expected_url.scheme.lower(),
            expected_url.username,
            expected_url.password,
            expected_url.hostname.lower() if expected_url.hostname else None,
            expected_url.port,
            expected_url.path,
            expected_url.query,
        )
    except ValueError:
        return False
    return candidate_identity == expected_identity


def _valid_redirect_uri(uri: str) -> bool:
    """OAuth 2.1 permits HTTPS redirects and HTTP only for loopback clients."""
    try:
        parsed = urlsplit(uri)
        if parsed.fragment or not parsed.hostname:
            return False
        if parsed.scheme.lower() == "https":
            return True
        if parsed.scheme.lower() != "http":
            return False
        host = parsed.hostname
        if host.lower() == "localhost":
            return True
        return ipaddress.ip_address(host).is_loopback
    except ValueError:
        return False


def _client_id(client: OAuthClientInformationFull) -> str:
    """`OAuthClientInformationFull.client_id` is typed optional (it is absent on the registration
    REQUEST, before the server assigns one). Everywhere this module touches a client the id has
    already been assigned, so a missing one is a broken invariant, not a case to paper over."""
    cid = client.client_id
    if not cid:
        raise ValueError("OAuth client reached the provider without a client_id")
    return cid


@dataclass
class _PendingConsent:
    """An /authorize request parked until the operator proves the passphrase."""

    client_id: str
    params: AuthorizationParams
    created_at: int = field(default_factory=_now)
    attempts: int = 0

    def expired(self) -> bool:
        return _now() - self.created_at > CONSENT_TXN_TTL


class HarvesterAuthProvider(
    OAuthAuthorizationServerProvider[AuthorizationCode, RefreshToken, AccessToken]
):
    """OAuth provider whose entire user model is "knows the operator passphrase".

    `static_token` is an escape hatch for non-browser clients (curl, Claude Code, a claude.ai
    account that has the request-header beta): a fixed bearer accepted verbatim. It is off unless
    configured, and it is compared in constant time.
    """

    def __init__(
        self,
        issuer_url: str,
        passphrase: str | None,
        state_path: Path,
        static_token: str | None = None,
        resource_url: str | None = None,
    ) -> None:
        self.issuer_url = issuer_url.rstrip("/")
        self.resource_url = resource_url or f"{self.issuer_url}/mcp"
        self._passphrase = passphrase
        self._static_token_hash = _hash(static_token) if static_token else None
        self._state_path = state_path
        self._clients: dict[str, OAuthClientInformationFull] = {}
        self._auth_codes: dict[str, AuthorizationCode] = {}       # in-memory: 5-min TTL
        self._access: dict[str, AccessToken] = {}                 # keyed by token HASH
        self._refresh: dict[str, RefreshToken] = {}               # keyed by token HASH
        self._pending: dict[str, _PendingConsent] = {}
        self._load()

    # ── persistence ──────────────────────────────────────────────────────────
    # Clients and refresh tokens survive a restart; access tokens and auth codes do not (both are
    # short-lived, and Claude re-mints them from the refresh token on the next 401).

    def _load(self) -> None:
        if not self._state_path.exists():
            return
        try:
            raw = json.loads(self._state_path.read_text(encoding="utf-8"))
        except (OSError, ValueError) as e:
            log.warning("auth state unreadable at %s (%s) — starting empty; "
                        "existing connectors will need to re-authorize", self._state_path, e)
            return
        try:
            for c in raw.get("clients", []):
                info = OAuthClientInformationFull.model_validate(c)
                self._clients[_client_id(info)] = info
            for r in raw.get("refresh", []):
                tok = RefreshToken.model_validate(r)
                self._refresh[r["_hash"]] = tok
        except (ValueError, KeyError, TypeError) as e:
            log.warning("auth state failed validation (%s) — starting empty", e)
            self._clients.clear()
            self._refresh.clear()
            return
        log.info("auth state loaded: %d client(s), %d refresh token(s)",
                 len(self._clients), len(self._refresh))

    def _save(self) -> None:
        payload = {
            "clients": [c.model_dump(mode="json", exclude_none=True) for c in self._clients.values()],
            "refresh": [
                {**t.model_dump(mode="json", exclude_none=True), "_hash": h}
                for h, t in self._refresh.items()
            ],
        }
        try:
            self._state_path.parent.mkdir(parents=True, exist_ok=True)
            tmp = self._state_path.with_suffix(".tmp")
            tmp.write_text(json.dumps(payload, indent=2), encoding="utf-8")
            os.chmod(tmp, 0o600)          # before the rename — never briefly world-readable
            tmp.replace(self._state_path)
        except OSError as e:
            # Non-fatal: the server keeps working from memory, but say so loudly — the operator
            # will otherwise discover it only as a mystery re-authorization after a restart.
            log.warning("could not persist auth state to %s: %s — "
                        "authorizations will not survive a restart", self._state_path, e)

    # ── client registration (DCR) ────────────────────────────────────────────

    async def get_client(self, client_id: str) -> OAuthClientInformationFull | None:
        return self._clients.get(client_id)

    async def register_client(self, client_info: OAuthClientInformationFull) -> None:
        if client_info.token_endpoint_auth_method not in {
            "none", "client_secret_post", "client_secret_basic",
        }:
            raise RegistrationError(
                "invalid_client_metadata",
                "token_endpoint_auth_method is not supported",
            )
        invalid_redirects = [
            str(uri) for uri in client_info.redirect_uris or []
            if not _valid_redirect_uri(str(uri))
        ]
        if invalid_redirects:
            raise RegistrationError(
                "invalid_redirect_uri",
                "redirect_uris must use HTTPS or HTTP loopback URLs without fragments",
            )
        # RFC 7591 §3.2.1 requires this field whenever a secret is issued; zero means that the
        # secret does not expire. The SDK otherwise omits it for its non-expiring default.
        if client_info.client_secret and client_info.client_secret_expires_at is None:
            client_info.client_secret_expires_at = 0
        self._clients[_client_id(client_info)] = client_info
        self._save()
        log.info("registered client %s redirect_uris=%s",
                 client_info.client_id, [str(u) for u in client_info.redirect_uris or []])

    # ── authorization ────────────────────────────────────────────────────────

    async def authorize(
        self, client: OAuthClientInformationFull, params: AuthorizationParams
    ) -> str:
        """Park the request and send the browser to the consent page.

        No code is minted here — the caller has proven nothing yet. `remote.py` serves /consent,
        which calls `complete_consent` once the passphrase checks out.
        """
        if params.resource is not None and not resource_matches(params.resource, self.resource_url):
            # The route middleware emits RFC 8707's `invalid_target`; keep this provider-level
            # rejection as defense in depth for callers that bypass the HTTP handler.
            raise AuthorizeError("invalid_request", "resource does not identify this MCP server")
        if not _PKCE_CHALLENGE.fullmatch(params.code_challenge):
            raise AuthorizeError("invalid_request", "code_challenge is not valid PKCE")
        # 2025-03-26 clients predate MCP's mandatory resource parameter. RFC 8707 §2.1 explicitly
        # permits a predefined default, so bind those legacy requests to this server too.
        params.resource = self.resource_url
        txn = secrets.token_urlsafe(24)
        self._pending[txn] = _PendingConsent(client_id=_client_id(client), params=params)
        self._sweep()
        log.info("authorize: client=%s txn=%s resource=%s",
                 client.client_id, txn[:8], params.resource)
        return f"{self.issuer_url}/consent?txn={txn}"

    def pending(self, txn: str) -> _PendingConsent | None:
        p = self._pending.get(txn)
        if p and p.expired():
            del self._pending[txn]
            return None
        return p

    def check_passphrase(self, txn: str, supplied: str) -> bool:
        """Constant-time passphrase check, with a per-transaction attempt cap so a parked txn
        can't be brute-forced by replaying its URL."""
        p = self.pending(txn)
        if p is None or self._passphrase is None:
            return False
        p.attempts += 1
        if p.attempts > MAX_CONSENT_ATTEMPTS:
            del self._pending[txn]
            log.warning("consent txn %s burned after %d failed attempts", txn[:8], p.attempts - 1)
            return False
        ok = secrets.compare_digest(supplied, self._passphrase)
        if not ok:
            log.warning("failed consent attempt %d/%d for txn %s",
                        p.attempts, MAX_CONSENT_ATTEMPTS, txn[:8])
        return ok

    def complete_consent(self, txn: str) -> str:
        """Mint the authorization code and return the redirect URL back to the client.

        Only call after `check_passphrase` returned True.
        """
        p = self._pending.pop(txn)
        code = secrets.token_urlsafe(32)
        self._auth_codes[code] = AuthorizationCode(
            code=code,
            scopes=p.params.scopes or [SCOPE],
            expires_at=time.time() + AUTH_CODE_TTL,
            client_id=p.client_id,
            code_challenge=p.params.code_challenge,       # PKCE verified by the SDK at /token
            redirect_uri=p.params.redirect_uri,
            redirect_uri_provided_explicitly=p.params.redirect_uri_provided_explicitly,
            resource=p.params.resource,
        )
        log.info("consent granted: client=%s txn=%s", p.client_id, txn[:8])
        return construct_redirect_uri(
            str(p.params.redirect_uri), code=code, state=p.params.state
        )

    async def load_authorization_code(
        self, client: OAuthClientInformationFull, authorization_code: str
    ) -> AuthorizationCode | None:
        code = self._auth_codes.get(authorization_code)
        if code is None:
            return None
        if code.client_id != client.client_id or code.expires_at < time.time():
            # Wrong client or expired — drop it rather than leave a stale code lying around.
            self._auth_codes.pop(authorization_code, None)
            return None
        return code

    async def exchange_authorization_code(
        self, client: OAuthClientInformationFull, authorization_code: AuthorizationCode
    ) -> OAuthToken:
        # Single use: burn the code before issuing, so a replayed exchange finds nothing.
        self._auth_codes.pop(authorization_code.code, None)
        return self._issue(_client_id(client), authorization_code.scopes, authorization_code.resource)

    # ── tokens ───────────────────────────────────────────────────────────────

    def _issue(self, client_id: str, scopes: list[str], resource: str | None) -> OAuthToken:
        access = secrets.token_urlsafe(32)
        refresh = secrets.token_urlsafe(32)
        # This authorization server has exactly one protected resource. Even legacy clients that
        # omit RFC 8707's parameter receive an audience-bound token for that resource.
        resource = self.resource_url
        # Access tokens are in-memory only and expire in an hour, so the model carries the real
        # value. Refresh tokens are PERSISTED, so the model carries the DIGEST in its `token` field
        # — the raw value never reaches disk. Safe because the SDK's TokenHandler reads only
        # `.client_id` and `.scopes` off a loaded refresh token, never `.token`; rotation below
        # relies on the same substitution.
        self._access[_hash(access)] = AccessToken(
            token=access, client_id=client_id, scopes=scopes,
            expires_at=_now() + ACCESS_TOKEN_TTL, resource=resource,
        )
        refresh_hash = _hash(refresh)
        self._refresh[refresh_hash] = RefreshToken(
            token=refresh_hash, client_id=client_id, scopes=scopes, expires_at=None,
        )
        self._save()
        return OAuthToken(
            access_token=access, token_type="Bearer",
            expires_in=ACCESS_TOKEN_TTL, scope=" ".join(scopes), refresh_token=refresh,
        )

    async def load_refresh_token(
        self, client: OAuthClientInformationFull, refresh_token: str
    ) -> RefreshToken | None:
        tok = self._refresh.get(_hash(refresh_token))
        if tok is None or tok.client_id != client.client_id:
            return None
        return tok

    async def exchange_refresh_token(
        self,
        client: OAuthClientInformationFull,
        refresh_token: RefreshToken,
        scopes: list[str],
    ) -> OAuthToken:
        # ROTATION: the old refresh token dies in the same response that mints its replacement.
        # OAuth 2.1 requires this for public clients, and DCR registers Claude as one.
        # `.token` already holds the digest (see `_issue`) — do NOT hash it again.
        self._refresh.pop(refresh_token.token, None)
        return self._issue(
            _client_id(client), scopes or refresh_token.scopes, resource=self.resource_url
        )

    async def load_access_token(self, token: str) -> AccessToken | None:
        # Static operator token first — a fixed credential for non-browser clients. Compared by
        # hash digest (constant time) so a timing side channel can't walk it out.
        if self._static_token_hash is not None and secrets.compare_digest(
            _hash(token), self._static_token_hash
        ):
            return AccessToken(
                token=token, client_id="static", scopes=[SCOPE], expires_at=None, resource=None
            )
        tok = self._access.get(_hash(token))
        if tok is None:
            return None
        if not tok.resource or not resource_matches(tok.resource, self.resource_url):
            return None
        if tok.expires_at is not None and tok.expires_at < _now():
            del self._access[_hash(token)]
            return None
        return tok

    async def verify_token(self, token: str) -> AccessToken | None:
        """TokenVerifier protocol — the resource-server half of the same lookup."""
        return await self.load_access_token(token)

    async def revoke_token(self, token: Any) -> None:
        # An AccessToken carries its raw value, a RefreshToken carries its digest (see `_issue`),
        # so try both forms rather than assuming which kind arrived.
        value = getattr(token, "token", "")
        for key in (value, _hash(value)):
            self._access.pop(key, None)
            self._refresh.pop(key, None)
        self._save()

    # ── housekeeping ─────────────────────────────────────────────────────────

    def _sweep(self) -> None:
        """Drop expired pending consents, auth codes, and access tokens. Cheap, and it keeps a
        long-lived process from accumulating dead state."""
        for txn in [t for t, p in self._pending.items() if p.expired()]:
            del self._pending[txn]
        now = time.time()
        for code in [c for c, a in self._auth_codes.items() if a.expires_at < now]:
            del self._auth_codes[code]
        for h in [
            h for h, a in self._access.items()
            if a.expires_at is not None and a.expires_at < now
        ]:
            del self._access[h]
