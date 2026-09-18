package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"drone-mcp-server/tool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testToken = "correct-horse-battery-staple"

func TestBearerToken(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
		ok     bool
	}{
		{"missing", "", "", false},
		{"wrong scheme", "Basic abc", "", false},
		{"empty credentials", "Bearer ", "", false},
		{"blank credentials", "Bearer    ", "", false},
		{"valid", "Bearer " + testToken, testToken, true},
		{"lowercase scheme", "bearer " + testToken, testToken, true},
		{"mixed case scheme", "BeArEr " + testToken, testToken, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := bearerToken(tc.header)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("bearerToken(%q) = (%q, %v), want (%q, %v)", tc.header, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestAuthMiddlewareStatusCodes(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"missing header", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic abc", http.StatusUnauthorized},
		{"empty credentials", "Bearer ", http.StatusUnauthorized},
		{"wrong token", "Bearer not-the-token", http.StatusUnauthorized},
		{"token prefix only", "Bearer correct-horse-battery-stapl", http.StatusUnauthorized},
		{"valid token", "Bearer " + testToken, http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh limiter per case keeps the statuses independent.
			handler := authMiddleware(okHandler(), testToken, newAuthLimiter(authFailureLimit, authFailureWindow))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestAuthMiddlewareSetsWWWAuthenticate(t *testing.T) {
	handler := authMiddleware(okHandler(), testToken, newAuthLimiter(authFailureLimit, authFailureWindow))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Bearer") {
		t.Fatalf("WWW-Authenticate = %q, want it to mention Bearer", got)
	}
}

func TestAuthMiddlewareThrottlesFailures(t *testing.T) {
	limiter := newAuthLimiter(2, time.Minute)
	handler := authMiddleware(okHandler(), testToken, limiter)

	fail := func() int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := fail(); code != http.StatusUnauthorized {
		t.Fatalf("first failure status = %d, want 401", code)
	}
	if code := fail(); code != http.StatusUnauthorized {
		t.Fatalf("second failure status = %d, want 401", code)
	}
	if code := fail(); code != http.StatusTooManyRequests {
		t.Fatalf("third failure status = %d, want 429", code)
	}

	// A valid token is also rejected while the address stays blocked.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked request status = %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Fatal("blocked response is missing Retry-After")
	}
}

func TestAuthMiddlewareSuccessResetsCounter(t *testing.T) {
	limiter := newAuthLimiter(2, time.Minute)
	handler := authMiddleware(okHandler(), testToken, limiter)

	request := func(header string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := request("Bearer wrong"); code != http.StatusUnauthorized {
		t.Fatalf("failure status = %d, want 401", code)
	}
	if code := request("Bearer " + testToken); code != http.StatusNoContent {
		t.Fatalf("valid status = %d, want 204", code)
	}
	if code := request("Bearer wrong"); code != http.StatusUnauthorized {
		t.Fatalf("failure after success status = %d, want 401 (counter should reset)", code)
	}
}

func TestAuthLimiterWindowExpiry(t *testing.T) {
	limiter := newAuthLimiter(1, time.Minute)
	now := time.Now()
	limiter.now = func() time.Time { return now }

	limiter.failure("10.0.0.1")
	if !limiter.blocked("10.0.0.1") {
		t.Fatal("address should be blocked after reaching the limit")
	}

	now = now.Add(2 * time.Minute)
	if limiter.blocked("10.0.0.1") {
		t.Fatal("address should be unblocked after the window expires")
	}
}

func TestHostGuard(t *testing.T) {
	guard := hostGuard(okHandler(), []string{"mcp.example.com", "127.0.0.1"})

	cases := []struct {
		host string
		want int
	}{
		{"mcp.example.com", http.StatusNoContent},
		{"mcp.example.com:8080", http.StatusNoContent},
		{"MCP.Example.COM:8443", http.StatusNoContent},
		{"127.0.0.1:8080", http.StatusNoContent},
		{"attacker.example", http.StatusForbidden},
		{"attacker.example:8080", http.StatusForbidden},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Host = tc.host
		guard.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("Host %q: status = %d, want %d", tc.host, rec.Code, tc.want)
		}
	}
}

func TestHostGuardDisabledByDefault(t *testing.T) {
	guard := hostGuard(okHandler(), nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "anything.example"
	guard.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want the empty allowlist to disable the check", rec.Code)
	}
}

func TestHostPolicy(t *testing.T) {
	cases := []struct {
		name                string
		localhostProtection bool
		explicitlySet       bool
		allowedHosts        []string
		wantLoopback        bool
		wantNote            string
	}{
		{
			name:                "default keeps the loopback heuristic",
			localhostProtection: true,
			wantLoopback:        true,
		},
		{
			name:                "allowlist replaces the heuristic",
			localhostProtection: true,
			allowedHosts:        []string{"mcp.example.com"},
			wantLoopback:        false,
			wantNote:            "instead of the transport's loopback Host heuristic",
		},
		{
			name:                "explicit true stacks both checks",
			localhostProtection: true,
			explicitlySet:       true,
			allowedHosts:        []string{"mcp.example.com"},
			wantLoopback:        true,
		},
		{
			name:                "disabled without allowlist warns",
			localhostProtection: false,
			explicitlySet:       true,
			wantLoopback:        false,
			wantNote:            "WARNING:",
		},
		{
			name:                "disabled with allowlist",
			localhostProtection: false,
			explicitlySet:       true,
			allowedHosts:        []string{"mcp.example.com"},
			wantLoopback:        false,
			wantNote:            "compensating control",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loopback, note := hostPolicy(tc.localhostProtection, tc.explicitlySet, tc.allowedHosts)
			if loopback != tc.wantLoopback {
				t.Fatalf("loopbackCheck = %v, want %v", loopback, tc.wantLoopback)
			}
			if tc.wantNote == "" && note != "" {
				t.Fatalf("unexpected note %q", note)
			}
			if tc.wantNote != "" && !strings.Contains(note, tc.wantNote) {
				t.Fatalf("note = %q, want it to contain %q", note, tc.wantNote)
			}
		})
	}
}

func TestNormalizeHost(t *testing.T) {
	cases := map[string]string{
		"Example.COM:8080": "example.com",
		"example.com":      "example.com",
		"[::1]:8080":       "::1",
		"":                 "",
	}
	for in, want := range cases {
		if got := normalizeHost(in); got != want {
			t.Fatalf("normalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClientIPIgnoresForwardedHeadersByDefault(t *testing.T) {
	newRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.5:1234"
		req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
		req.Header.Set("X-Real-IP", "9.9.9.9")
		return req
	}

	if got := clientIP(newRequest(), false); got != "10.0.0.5" {
		t.Fatalf("clientIP(trustProxy=false) = %q, want the socket address", got)
	}
	if got := clientIP(newRequest(), true); got != "1.2.3.4" {
		t.Fatalf("clientIP(trustProxy=true) = %q, want the first forwarded address", got)
	}

	req := newRequest()
	req.Header.Del("X-Forwarded-For")
	if got := clientIP(req, true); got != "9.9.9.9" {
		t.Fatalf("clientIP fallback = %q, want X-Real-IP", got)
	}
}

func TestSanitizeLogStripsControlCharacters(t *testing.T) {
	got := sanitizeLog("/path%0a2026/01/01 [AUTH] forged\r\nline\tend")
	for _, bad := range []string{"\n", "\r"} {
		if strings.Contains(got, bad) {
			t.Fatalf("sanitizeLog left a control character in %q", got)
		}
	}
	if !strings.Contains(got, "forged") {
		t.Fatalf("sanitizeLog dropped the payload entirely: %q", got)
	}
}

func TestSanitizeLogTruncates(t *testing.T) {
	got := sanitizeLog(strings.Repeat("a", maxLogValueLength+50))
	if !strings.HasSuffix(got, "...(truncated)") {
		t.Fatalf("sanitizeLog did not mark truncation: %q", got)
	}
	if runes := []rune(strings.TrimSuffix(got, "...(truncated)")); len(runes) != maxLogValueLength {
		t.Fatalf("truncated length = %d, want %d", len(runes), maxLogValueLength)
	}
}

func TestSplitList(t *testing.T) {
	got := splitList(" a, b ,,c ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitList returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitList returned %v, want %v", got, want)
		}
	}
}

func TestResolveAuthToken(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		t.Setenv("MCP_AUTH_TOKEN", "")
		t.Setenv("MCP_AUTH_TOKEN_FILE", "")
		token, source, err := resolveAuthToken()
		if err != nil || token != "" || source != "" {
			t.Fatalf("resolveAuthToken() = (%q, %q, %v), want empty result", token, source, err)
		}
	})

	t.Run("from environment", func(t *testing.T) {
		t.Setenv("MCP_AUTH_TOKEN_FILE", "")
		t.Setenv("MCP_AUTH_TOKEN", "  env-token  ")
		token, source, err := resolveAuthToken()
		if err != nil || token != "env-token" || source != "MCP_AUTH_TOKEN" {
			t.Fatalf("resolveAuthToken() = (%q, %q, %v), want the trimmed env token", token, source, err)
		}
	})

	t.Run("from file, taking precedence", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token")
		if err := os.WriteFile(path, []byte("file-token\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("MCP_AUTH_TOKEN", "env-token")
		t.Setenv("MCP_AUTH_TOKEN_FILE", path)
		token, source, err := resolveAuthToken()
		if err != nil || token != "file-token" || source != "MCP_AUTH_TOKEN_FILE" {
			t.Fatalf("resolveAuthToken() = (%q, %q, %v), want the file token", token, source, err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		t.Setenv("MCP_AUTH_TOKEN", "")
		t.Setenv("MCP_AUTH_TOKEN_FILE", filepath.Join(t.TempDir(), "absent"))
		if _, _, err := resolveAuthToken(); err == nil {
			t.Fatal("resolveAuthToken() succeeded for a missing file, want an error")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token")
		if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("MCP_AUTH_TOKEN", "")
		t.Setenv("MCP_AUTH_TOKEN_FILE", path)
		if _, _, err := resolveAuthToken(); err == nil {
			t.Fatal("resolveAuthToken() succeeded for an empty file, want an error")
		}
	})
}

func TestWriteToolsEnabled(t *testing.T) {
	previous := *enableWriteTools
	t.Cleanup(func() { *enableWriteTools = previous })

	cases := []struct {
		flag bool
		env  string
		want bool
	}{
		{false, "", false},
		{false, "true", true},
		{false, "1", true},
		{false, "on", true},
		{false, "no", false},
		{true, "", true},
	}
	for _, tc := range cases {
		*enableWriteTools = tc.flag
		t.Setenv("MCP_ENABLE_WRITE_TOOLS", tc.env)
		if got := writeToolsEnabled(); got != tc.want {
			t.Fatalf("writeToolsEnabled(flag=%v, env=%q) = %v, want %v", tc.flag, tc.env, got, tc.want)
		}
	}
}

// TestToolRegistrationReadOnlyMode is the regression test for the destructive
// tool surface: without explicit write access only read-only tools may be
// exposed to a client.
func TestToolRegistrationReadOnlyMode(t *testing.T) {
	previous := *enableWriteTools
	t.Cleanup(func() { *enableWriteTools = previous })

	t.Run("read-only by default", func(t *testing.T) {
		*enableWriteTools = false
		tools := listRegisteredTools(t)

		if len(tools) != 18 {
			t.Fatalf("registered %d tools, want 18 read-only tools", len(tools))
		}
		for _, registered := range tools {
			if registered.Annotations == nil || !registered.Annotations.ReadOnlyHint {
				t.Fatalf("tool %q is not annotated as read-only", registered.Name)
			}
		}
		for _, forbidden := range []string{"delete_user", "delete_org_secret", "chown_repo", "create_secret", "cancel_build"} {
			if _, ok := tools[forbidden]; ok {
				t.Fatalf("write tool %q is exposed in read-only mode", forbidden)
			}
		}
	})

	t.Run("write tools when enabled", func(t *testing.T) {
		*enableWriteTools = true
		tools := listRegisteredTools(t)

		if len(tools) != 45 {
			t.Fatalf("registered %d tools, want 45", len(tools))
		}
		for _, name := range []string{"delete_user", "chown_repo", "promote_build", "create_secret"} {
			registered, ok := tools[name]
			if !ok {
				t.Fatalf("write tool %q is missing", name)
			}
			if registered.Annotations == nil || registered.Annotations.ReadOnlyHint {
				t.Fatalf("write tool %q is annotated as read-only", name)
			}
		}

		destructive := tools["delete_user"]
		if destructive.Annotations.DestructiveHint == nil || !*destructive.Annotations.DestructiveHint {
			t.Fatal("delete_user should be annotated as destructive")
		}
		additive := tools["create_secret"]
		if additive.Annotations.DestructiveHint == nil || *additive.Annotations.DestructiveHint {
			t.Fatal("create_secret should not be annotated as destructive")
		}
	})
}

// TestToolArgumentsAreValidated proves the registration wrapper rejects
// path-shaping arguments before they reach the Drone client.
func TestToolArgumentsAreValidated(t *testing.T) {
	previous := *enableWriteTools
	*enableWriteTools = false
	t.Cleanup(func() { *enableWriteTools = previous })

	clientSession := connectTestClient(t)

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_repo",
		Arguments: map[string]any{"owner": "../users", "repo": "drone"},
	})
	if err == nil && (result == nil || !result.IsError) {
		t.Fatal("get_repo accepted a traversing owner name")
	}
}

func TestToolCallName(t *testing.T) {
	name, ok := toolCallName("tools/call", &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: "list_repos"},
	})
	if !ok || name != "list_repos" {
		t.Fatalf("toolCallName() = (%q, %v), want list_repos", name, ok)
	}
	if _, ok := toolCallName("tools/list", &mcp.CallToolRequest{}); ok {
		t.Fatal("toolCallName reported a tool for tools/list")
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

// newTestServer builds a server with the production tool set. The Drone client
// is nil: these tests only inspect the registered tool surface, they never
// invoke a handler that talks to Drone.
func newTestServer() *mcp.Server {
	ds := &DroneServer{}
	ds.repoHandler = tool.NewRepoHandler(ds.client)
	ds.buildHandler = tool.NewBuildHandler(ds.client)
	ds.resourceHandler = tool.NewResourceHandler(ds.client)
	ds.cronHandler = tool.NewCronHandler(ds.client)
	ds.secretHandler = tool.NewSecretHandler(ds.client)
	ds.userHandler = tool.NewUserHandler(ds.client)
	ds.templateHandler = tool.NewTemplateHandler(ds.client)

	server := mcp.NewServer(&mcp.Implementation{Name: "drone-mcp-server-test", Version: "test"}, nil)
	hiddenWriteTools = 0
	registerTools(server, ds)
	return server
}

func connectTestClient(t *testing.T) *mcp.ClientSession {
	t.Helper()

	server := newTestServer()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { clientSession.Close() })

	return clientSession
}

func listRegisteredTools(t *testing.T) map[string]*mcp.Tool {
	t.Helper()

	clientSession := connectTestClient(t)
	ctx := context.Background()

	tools := make(map[string]*mcp.Tool)
	var cursor string
	for {
		result, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			t.Fatalf("ListTools: %v", err)
		}
		for _, registered := range result.Tools {
			tools[registered.Name] = registered
		}
		if result.NextCursor == "" {
			return tools
		}
		cursor = result.NextCursor
	}
}
