package harvestmcp

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func remoteRequest(t *testing.T, server *RemoteServer, method, path, body, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "https://harvester.example.test"+path, strings.NewReader(body))
	req.Host = "harvester.example.test"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func TestRemoteOpenAndStaticGateway(t *testing.T) {
	base := t.TempDir()
	open, err := NewRemote(RemoteOptions{Runtime: Runtime{Home: base, CacheDir: base + "/cache"}, PublicURL: "https://harvester.example.test", DisableConfinement: false})
	if err != nil {
		t.Fatal(err)
	}
	if rec := remoteRequest(t, open, http.MethodGet, "/healthz", "", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"auth":false`) {
		t.Fatalf("open health = %d %s", rec.Code, rec.Body.String())
	}
	if rec := remoteRequest(t, open, http.MethodGet, "/mcp/", "", ""); rec.Code == http.StatusPermanentRedirect || rec.Code == http.StatusTemporaryRedirect {
		t.Fatalf("/mcp/ unexpectedly redirected: %d", rec.Code)
	}
	static, err := NewRemote(RemoteOptions{Runtime: Runtime{Home: base, CacheDir: base + "/cache"}, PublicURL: "https://harvester.example.test", StaticToken: "fixture-token"})
	if err != nil {
		t.Fatal(err)
	}
	rec := remoteRequest(t, static, http.MethodGet, "/mcp", "", "")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Header().Get("WWW-Authenticate"), "resource_metadata") {
		t.Fatalf("static challenge = %d %s", rec.Code, rec.Header().Get("WWW-Authenticate"))
	}
	req := httptest.NewRequest(http.MethodGet, "https://harvester.example.test/mcp", nil)
	req.Host = "harvester.example.test"
	req.Header.Set("Authorization", "Bearer fixture-token")
	rec = httptest.NewRecorder()
	static.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("static token rejected: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRemoteOAuthPKCEAndRefreshRotationInMemory(t *testing.T) {
	base := t.TempDir()
	server, err := NewRemote(RemoteOptions{Runtime: Runtime{Home: base, CacheDir: base + "/cache"}, PublicURL: "https://harvester.example.test", Passphrase: "fixture-pass", StatePath: base + "/state/auth.json"})
	if err != nil {
		t.Fatal(err)
	}
	reg := remoteRequest(t, server, http.MethodPost, "/register", `{"client_name":"fixture","redirect_uris":["https://client.example/callback"],"token_endpoint_auth_method":"none"}`, "application/json")
	if reg.Code != http.StatusCreated {
		t.Fatalf("register = %d %s", reg.Code, reg.Body.String())
	}
	var client map[string]any
	if err := json.Unmarshal(reg.Body.Bytes(), &client); err != nil {
		t.Fatal(err)
	}
	clientID, _ := client["client_id"].(string)
	verifier := strings.Repeat("v", 43)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	query := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {"https://client.example/callback"}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"s"}, "scope": {HarvesterScope}}
	auth := remoteRequest(t, server, http.MethodGet, "/authorize?"+query.Encode(), "", "")
	if auth.Code != http.StatusFound || !strings.Contains(auth.Header().Get("Location"), "/consent?txn=") {
		t.Fatalf("authorize = %d %s", auth.Code, auth.Header().Get("Location"))
	}
	txn := strings.TrimPrefix(auth.Header().Get("Location"), "https://harvester.example.test/consent?txn=")
	consent := remoteRequest(t, server, http.MethodPost, "/consent", "txn="+url.QueryEscape(txn)+"&passphrase=fixture-pass", "application/x-www-form-urlencoded")
	if consent.Code != http.StatusFound {
		t.Fatalf("consent = %d %s", consent.Code, consent.Body.String())
	}
	callback, err := url.Parse(consent.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	tokenBody := url.Values{"grant_type": {"authorization_code"}, "code": {callback.Query().Get("code")}, "client_id": {clientID}, "redirect_uri": {"https://client.example/callback"}, "code_verifier": {verifier}}.Encode()
	token := remoteRequest(t, server, http.MethodPost, "/token", tokenBody, "application/x-www-form-urlencoded")
	if token.Code != http.StatusOK {
		t.Fatalf("token = %d %s", token.Code, token.Body.String())
	}
	var first tokenResponse
	if err := json.Unmarshal(token.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	refreshBody := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {first.RefreshToken}, "client_id": {clientID}}.Encode()
	rotated := remoteRequest(t, server, http.MethodPost, "/token", refreshBody, "application/x-www-form-urlencoded")
	if rotated.Code != http.StatusOK {
		t.Fatalf("refresh = %d %s", rotated.Code, rotated.Body.String())
	}
	old := remoteRequest(t, server, http.MethodPost, "/token", refreshBody, "application/x-www-form-urlencoded")
	if old.Code == http.StatusOK {
		t.Fatal("rotated refresh token remained usable")
	}
}

func TestRemoteBindAndResourceRules(t *testing.T) {
	if err := ValidateRemoteBind("0.0.0.0", false, false); err == nil || !strings.Contains(err.Error(), "unauthenticated") {
		t.Fatalf("public open bind error = %v", err)
	}
	if err := ValidateRemoteBind("127.0.0.1", false, false); err != nil {
		t.Fatal(err)
	}
	if ResourceMatches("HTTPS://Harvester.Example/mcp", "https://harvester.example/mcp") != true {
		t.Fatal("scheme/host case should match")
	}
	if ResourceMatches("https://harvester.example/mcp/", "https://harvester.example/mcp") {
		t.Fatal("trailing slash should not match")
	}
}

func TestRemoteGatewayPlanSkipsUnsafeSecondDoor(t *testing.T) {
	base := t.TempDir()
	open, err := NewRemote(RemoteOptions{Runtime: Runtime{Home: base, CacheDir: base + "/cache"}, PublicURL: "http://127.0.0.1:8081"})
	if err != nil {
		t.Fatal(err)
	}
	gateways, err := PlanRemoteGateways(open, "127.0.0.1", 8081, nil, "127.0.0.1", 8082, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(gateways) != 1 || gateways[0].Label != "external" {
		t.Fatalf("open plan = %#v", gateways)
	}
	authed, err := NewRemote(RemoteOptions{Runtime: Runtime{Home: base, CacheDir: base + "/cache"}, PublicURL: "https://harvester.example.test", Passphrase: "pass"})
	if err != nil {
		t.Fatal(err)
	}
	internal, err := authed.NewInternalRemote("http://127.0.0.1:8082")
	if err != nil {
		t.Fatal(err)
	}
	gateways, err = PlanRemoteGateways(authed, "0.0.0.0", 8081, internal, "127.0.0.1", 8082, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(gateways) != 2 || gateways[0].Label != "external" || gateways[1].Label != "internal" {
		t.Fatalf("authenticated plan = %#v", gateways)
	}
}

func TestRemoteMetadataAliasesAndRegistrationRules(t *testing.T) {
	base := t.TempDir()
	server, err := NewRemote(RemoteOptions{Runtime: Runtime{Home: base, CacheDir: base + "/cache"}, PublicURL: "https://harvester.example.test", Passphrase: "pass"})
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-authorization-server", "/.well-known/oauth-authorization-server/mcp", "/.well-known/openid-configuration", "/.well-known/openid-configuration/mcp"}
	var first []byte
	for _, path := range paths {
		rec := remoteRequest(t, server, http.MethodGet, path, "", "")
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("metadata %s = %d headers=%v", path, rec.Code, rec.Header())
		}
		if strings.Contains(path, "protected") {
			if first == nil {
				first = rec.Body.Bytes()
			}
		} else if strings.Contains(path, "authorization") && !strings.Contains(path, "openid") {
			if !strings.Contains(rec.Body.String(), `"code_challenge_methods_supported":["S256"]`) {
				t.Fatalf("auth metadata omitted S256: %s", rec.Body.String())
			}
		}
	}
	bare := remoteRequest(t, server, http.MethodGet, paths[0], "", "")
	suffixed := remoteRequest(t, server, http.MethodGet, paths[1], "", "")
	if string(bare.Body.Bytes()) != string(suffixed.Body.Bytes()) {
		t.Fatal("protected-resource metadata aliases drifted")
	}
	badType := remoteRequest(t, server, http.MethodPost, "/register", "redirect_uris=x", "application/x-www-form-urlencoded")
	if badType.Code != http.StatusBadRequest || !strings.Contains(badType.Body.String(), "invalid_client_metadata") {
		t.Fatalf("registration wrong content type = %d %s", badType.Code, badType.Body.String())
	}
	badJSON := remoteRequest(t, server, http.MethodPost, "/register", "{", "application/json")
	if badJSON.Code != http.StatusBadRequest || !strings.Contains(badJSON.Body.String(), "invalid_client_metadata") {
		t.Fatalf("registration malformed JSON = %d %s", badJSON.Code, badJSON.Body.String())
	}
	if len(first) == 0 {
		t.Fatal("protected-resource metadata was empty")
	}
}
