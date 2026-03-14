package server

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/corruptmemory/file-uploader-r3/internal/app"
	"github.com/corruptmemory/file-uploader-r3/internal/auth"
	"github.com/corruptmemory/file-uploader-r3/internal/csv"
	"github.com/corruptmemory/file-uploader-r3/internal/mock"
	"github.com/corruptmemory/file-uploader-r3/internal/setup"
)

//go:embed testdata/static
var testStaticFS embed.FS

func testStaticSubFS(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(testStaticFS, "testdata/static")
	if err != nil {
		t.Fatalf("fs.Sub testdata/static: %v", err)
	}
	return sub
}

// newTestApp creates an Application in the RunningApp state for testing.
func newTestApp(t *testing.T) *app.Application {
	t.Helper()
	builder := func(a *app.Application) (app.Stoppable, error) {
		return &stubRunningApp{stopCh: make(chan struct{})}, nil
	}
	return app.NewApplication(builder)
}

// stubRunningApp is a minimal RunningApp for routing tests.
type stubRunningApp struct {
	stopCh          chan struct{}
	finishedDetails *app.CSVFinishedFile
	unsubMu         sync.Mutex
	unsubscribed    bool
	unsubscribedID  string

	// Configurable responses for spec-14 tests
	finishedFiles   []app.CSVFinishedFile
	config          app.ApplicationConfig
	mfaRequired     bool
	runningState    *app.RunningState
	updatedConfig   *app.ApplicationConfig
	downloadContent string
}

func (s *stubRunningApp) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

func (s *stubRunningApp) Wait() { <-s.stopCh }

func (s *stubRunningApp) Subscribe() (*app.EventSubscription, error) {
	ch := make(chan app.DataUpdateEvent, 1)
	ch <- app.DataUpdateEvent{State: app.CSVProcessingState{}}
	return &app.EventSubscription{ID: "test-sub", Events: ch}, nil
}
func (s *stubRunningApp) Unsubscribe(id string) error {
	s.unsubMu.Lock()
	s.unsubscribed = true
	s.unsubscribedID = id
	s.unsubMu.Unlock()
	return nil
}
func (s *stubRunningApp) GetFinishedDetails(id string) (*app.CSVFinishedFile, error) {
	if s.finishedDetails != nil {
		return s.finishedDetails, nil
	}
	return nil, fmt.Errorf("not found")
}
func (s *stubRunningApp) GetState() (*app.RunningState, error) {
	if s.runningState != nil {
		return s.runningState, nil
	}
	return &app.RunningState{
		Started: time.Now(),
	}, nil
}
func (s *stubRunningApp) ProcessUploadedCSVFile(uploadedBy, originalFilename, localFilePath string) error {
	return nil
}
func (s *stubRunningApp) SearchFinished(status app.FinishedStatus, csvTypes []csv.CSVType, search string) ([]app.CSVFinishedFile, error) {
	if s.finishedFiles == nil {
		return nil, nil
	}
	// Apply filters like the real implementation would
	var result []app.CSVFinishedFile
	for _, f := range s.finishedFiles {
		if status == app.FinishedStatusSuccess && !f.Success {
			continue
		}
		if status == app.FinishedStatusFailure && f.Success {
			continue
		}
		if len(csvTypes) > 0 {
			match := false
			for _, ct := range csvTypes {
				if f.CSVType == ct {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(f.InFile.OriginalFilename), searchLower) &&
				!strings.Contains(strings.ToLower(f.InFile.UploadedBy), searchLower) {
				continue
			}
		}
		result = append(result, f)
	}
	return result, nil
}
func (s *stubRunningApp) GetConfig() (app.ApplicationConfig, error) {
	return s.config, nil
}
func (s *stubRunningApp) MFARequired() (bool, error) { return s.mfaRequired, nil }
func (s *stubRunningApp) UpdateConfig(config app.ApplicationConfig) error {
	s.updatedConfig = &config
	return nil
}
func (s *stubRunningApp) DownloadPlayersDB(orgPlayerHash, orgPlayerIDPepper string, response http.ResponseWriter) error {
	if s.downloadContent != "" {
		response.Write([]byte(s.downloadContent))
	}
	return nil
}

var _ app.RunningApp = (*stubRunningApp)(nil)

// testSigningKey is a shared signing key for test JWTs.
var testSigningKey = []byte("test-key")

// newTestWebApp creates a WebApp with MockAuthProvider for testing.
func newTestWebApp(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	application := newTestApp(t)
	authProvider := &mock.MockAuthProvider{}
	_, router := NewWebApp(application, authProvider, testSigningKey, t.TempDir(), "test-version", "", testStaticSubFS(t), false)
	ts := httptest.NewServer(router)
	cleanup := func() {
		ts.Close()
		application.Stop()
		application.Wait()
	}
	return ts, cleanup
}

// newTestWebAppWithBlacklist creates a WebApp and returns the WebApp struct for blacklist access.
func newTestWebAppWithBlacklist(t *testing.T) (*httptest.Server, *WebApp, func()) {
	t.Helper()
	application := newTestApp(t)
	authProvider := &mock.MockAuthProvider{}
	wa, router := NewWebApp(application, authProvider, testSigningKey, t.TempDir(), "test-version", "", testStaticSubFS(t), false)
	ts := httptest.NewServer(router)
	cleanup := func() {
		ts.Close()
		application.Stop()
		application.Wait()
	}
	return ts, wa, cleanup
}

// loginAndGetCookies performs a POST /login and returns the session cookies.
func loginAndGetCookies(t *testing.T, ts *httptest.Server) []*http.Cookie {
	t.Helper()
	formData := "username=testuser&password=testpass"
	req, _ := http.NewRequest("POST", ts.URL+"/login", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	resp.Body.Close()

	// With HX-Request header, login returns 200 with HX-Redirect instead of 303
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	return resp.Cookies()
}

func TestHealthEndpoint(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", string(body), "ok")
	}
}

func TestUploadEnforces50MBLimit(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	// Create a multipart body larger than 50MB
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("files", "large.csv")
	if err != nil {
		t.Fatalf("creating form file: %v", err)
	}
	// Write just over 50MB of data
	chunk := make([]byte, 1<<20) // 1 MB
	for i := range chunk {
		chunk[i] = 'x'
	}
	for i := 0; i < 51; i++ {
		if _, err := part.Write(chunk); err != nil {
			t.Fatalf("writing chunk %d: %v", i, err)
		}
	}
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for _, c := range cookies {
		req.AddCookie(c)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /upload: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized upload: status = %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

func TestUploadRequiresAuth(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test.csv")
	if err != nil {
		t.Fatalf("creating form file: %v", err)
	}
	_, _ = part.Write([]byte("header1,header2\nval1,val2\n"))
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /upload: %v", err)
	}
	resp.Body.Close()

	// Without session cookie, should redirect to login (303)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("unauthenticated upload: status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
}

func TestStaticAssetsServed(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/js/app.js")
	if err != nil {
		t.Fatalf("GET /js/app.js: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestStaticAssetsEmbeddedAndMIMETypes(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	tests := []struct {
		name         string
		path         string
		wantStatus   int
		wantContains string // substring in Content-Type
	}{
		{
			name:         "js/app.js serves with javascript MIME",
			path:         "/js/app.js",
			wantStatus:   http.StatusOK,
			wantContains: "javascript",
		},
		{
			name:         "js/sse.js serves with javascript MIME",
			path:         "/js/sse.js",
			wantStatus:   http.StatusOK,
			wantContains: "javascript",
		},
		{
			name:         "css/app.css serves with css MIME",
			path:         "/css/app.css",
			wantStatus:   http.StatusOK,
			wantContains: "css",
		},
		{
			name:         "css/tokens.css serves with css MIME",
			path:         "/css/tokens.css",
			wantStatus:   http.StatusOK,
			wantContains: "css",
		},
		{
			name:         "vendor/htmx.min.js serves with javascript MIME",
			path:         "/vendor/htmx.min.js",
			wantStatus:   http.StatusOK,
			wantContains: "javascript",
		},
		{
			name:       "favicon.ico serves with icon MIME",
			path:       "/favicon.ico",
			wantStatus: http.StatusOK,
			wantContains: "icon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tt.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			ct := resp.Header.Get("Content-Type")
			if !strings.Contains(ct, tt.wantContains) {
				t.Errorf("Content-Type = %q, want it to contain %q", ct, tt.wantContains)
			}

			// Verify the body is non-empty (asset is actually embedded)
			body, _ := io.ReadAll(resp.Body)
			if len(body) == 0 {
				t.Errorf("body is empty for %s", tt.path)
			}
		})
	}
}

func TestHealthEndpointReturnsOK(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if got := strings.TrimSpace(string(body)); got != "ok" {
		t.Errorf("body = %q, want %q", got, "ok")
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
}

func TestLoginAndSessionCookies(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	var hasSession, hasSessionExpires bool
	for _, c := range cookies {
		switch c.Name {
		case "session":
			hasSession = true
		case "session-expires":
			hasSessionExpires = true
		}
	}
	if !hasSession {
		t.Error("login did not set session cookie")
	}
	if !hasSessionExpires {
		t.Error("login did not set session-expires cookie")
	}
}

func TestLoginUsesAuthProvider(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	// Parse the JWT to verify it has the mock org ID, not "pending"
	var sessionToken string
	for _, c := range cookies {
		if c.Name == "session" {
			sessionToken = c.Value
			break
		}
	}
	if sessionToken == "" {
		t.Fatal("no session cookie found")
	}

	claims, err := auth.ParseToken(sessionToken, testSigningKey)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}

	if claims.OrgID == "pending" {
		t.Error("OrgID is 'pending' — auth provider was not called")
	}
	if claims.OrgID != "mock-org-001" {
		t.Errorf("OrgID = %q, want %q", claims.OrgID, "mock-org-001")
	}
	if claims.Username != "testuser" {
		t.Errorf("Username = %q, want %q", claims.Username, "testuser")
	}
}

func TestSessionExtensionWithinWindow(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	// Create a token that expires in 30 seconds (within 40s window)
	claims := auth.JWTClaims{
		Username: "alice",
		OrgID:    "mock-org-001",
		JTI:      "test-jti-within",
		Exp:      time.Now().Add(30 * time.Second).Unix(),
	}
	tokenStr, err := auth.CreateToken(claims, testSigningKey)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/extend", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: tokenStr})

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /api/extend: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "extended" {
		t.Errorf("body = %q, want %q", string(body), "extended")
	}

	// Verify new session cookie was set
	var newSession *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			newSession = c
			break
		}
	}
	if newSession == nil {
		t.Fatal("no new session cookie set on extension")
	}

	// Parse the new token and verify it has a fresh expiry
	newClaims, err := auth.ParseToken(newSession.Value, testSigningKey)
	if err != nil {
		t.Fatalf("ParseToken new token: %v", err)
	}
	newExpiresIn := time.Until(time.Unix(newClaims.Exp, 0))
	if newExpiresIn < 4*time.Minute {
		t.Errorf("extended token expires too soon: %v", newExpiresIn)
	}
}

func TestSessionExtensionOutsideWindow(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	// Create a token that expires in 60 seconds (outside 40s window)
	claims := auth.JWTClaims{
		Username: "alice",
		OrgID:    "mock-org-001",
		JTI:      "test-jti-outside",
		Exp:      time.Now().Add(60 * time.Second).Unix(),
	}
	tokenStr, err := auth.CreateToken(claims, testSigningKey)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/extend", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: tokenStr})

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /api/extend: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", string(body), "ok")
	}

	// No new session cookie should be set
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			t.Error("session cookie should NOT be set outside extension window")
		}
	}
}

func TestWithStateOptionalSession(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	// The withStateOptionalSession middleware is used on the login GET route
	// to demonstrate it's exercised. We test it directly by hitting a route
	// that doesn't require auth but the app needs to be in running state.
	// Login GET is already a running-state route, so we verify it works
	// without a session cookie.
	resp, err := http.Get(ts.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestSessionReplayAfterLogout(t *testing.T) {
	ts, wa, cleanup := newTestWebAppWithBlacklist(t)
	defer cleanup()

	// Login to get a valid session token
	cookies := loginAndGetCookies(t, ts)

	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie after login")
	}

	// Verify the token works before logout
	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.AddCookie(sessionCookie)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET / before logout: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard before logout: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Perform logout (POST)
	logoutReq, _ := http.NewRequest("POST", ts.URL+"/logout", nil)
	logoutReq.Header.Set("HX-Request", "true")
	logoutReq.AddCookie(sessionCookie)
	logoutResp, err := client.Do(logoutReq)
	if err != nil {
		t.Fatalf("POST /logout: %v", err)
	}
	logoutResp.Body.Close()

	// Verify blacklist has the token's JTI
	claims, _ := auth.ParseToken(sessionCookie.Value, testSigningKey)
	if claims == nil {
		t.Fatal("could not parse session token")
	}
	if !wa.tokenBlacklist.IsRevoked(claims.JTI) {
		t.Error("token JTI should be in blacklist after logout")
	}

	// Try to replay the old token — should be rejected (redirect to login)
	replayReq, _ := http.NewRequest("GET", ts.URL+"/", nil)
	replayReq.AddCookie(sessionCookie)
	replayResp, err := client.Do(replayReq)
	if err != nil {
		t.Fatalf("GET / replay after logout: %v", err)
	}
	replayResp.Body.Close()

	if replayResp.StatusCode != http.StatusSeeOther {
		t.Errorf("replayed token after logout: status = %d, want %d (redirect to login)", replayResp.StatusCode, http.StatusSeeOther)
	}
}

func TestSecurityHeadersPresent(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	tests := []struct {
		header string
		want   string
	}{
		{"X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "nosniff"},
		{"Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'"},
	}

	for _, tt := range tests {
		got := resp.Header.Get(tt.header)
		if got != tt.want {
			t.Errorf("%s = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestLogoutRequiresPost(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// GET /logout should return 405 Method Not Allowed
	req, _ := http.NewRequest("GET", ts.URL+"/logout", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /logout: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /logout: status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}

	// POST /logout should work (redirect to login)
	req, _ = http.NewRequest("POST", ts.URL+"/logout", nil)
	req.Header.Set("HX-Request", "true")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST /logout: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("POST /logout: status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
}

func TestSetupPostRequiresHXRequestHeader(t *testing.T) {
	// Create application in SetupApp state
	builder := func(a *app.Application) (app.Stoppable, error) {
		runningBuilder := func(a *app.Application) (app.Stoppable, error) {
			return &stubRunningApp{stopCh: make(chan struct{})}, nil
		}
		configWriter := func(ac app.ApplicationConfig) error { return nil }
		return setup.NewSetupApp(a, app.ApplicationConfig{}, &mock.MockAuthProvider{}, configWriter, runningBuilder, ""), nil
	}
	application := app.NewApplication(builder)
	defer func() {
		application.Stop()
		application.Wait()
	}()

	authProvider := &mock.MockAuthProvider{}
	_, router := NewWebApp(application, authProvider, testSigningKey, t.TempDir(), "test-version", "", testStaticSubFS(t), false)
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// POST without HX-Request header should return 403
	req, _ := http.NewRequest("POST", ts.URL+"/setup/next", strings.NewReader("current_step=0"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /setup/next without HX-Request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	// POST with HX-Request header should work (200)
	req, _ = http.NewRequest("POST", ts.URL+"/setup/next", strings.NewReader("current_step=0"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST /setup/next with HX-Request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestSetupPostRateLimiting(t *testing.T) {
	// Create application in SetupApp state
	builder := func(a *app.Application) (app.Stoppable, error) {
		runningBuilder := func(a *app.Application) (app.Stoppable, error) {
			return &stubRunningApp{stopCh: make(chan struct{})}, nil
		}
		configWriter := func(ac app.ApplicationConfig) error { return nil }
		return setup.NewSetupApp(a, app.ApplicationConfig{}, &mock.MockAuthProvider{}, configWriter, runningBuilder, ""), nil
	}
	application := app.NewApplication(builder)
	defer func() {
		application.Stop()
		application.Wait()
	}()

	authProvider := &mock.MockAuthProvider{}
	_, router := NewWebApp(application, authProvider, testSigningKey, t.TempDir(), "test-version", "", testStaticSubFS(t), false)
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Send 10 requests (should all succeed)
	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest("POST", ts.URL+"/setup/next", strings.NewReader("current_step=0"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i, resp.StatusCode, http.StatusOK)
		}
	}

	// 11th request should be rate limited
	req, _ := http.NewRequest("POST", ts.URL+"/setup/next", strings.NewReader("current_step=0"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("rate limited request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("rate limited: status = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
}

func TestRedirectToSetupWhenInSetupState(t *testing.T) {
	// Create application in SetupApp state
	builder := func(a *app.Application) (app.Stoppable, error) {
		runningBuilder := func(a *app.Application) (app.Stoppable, error) {
			return &stubRunningApp{stopCh: make(chan struct{})}, nil
		}
		configWriter := func(ac app.ApplicationConfig) error { return nil }
		return setup.NewSetupApp(a, app.ApplicationConfig{}, &mock.MockAuthProvider{}, configWriter, runningBuilder, ""), nil
	}
	application := app.NewApplication(builder)
	defer func() {
		application.Stop()
		application.Wait()
	}()

	authProvider := &mock.MockAuthProvider{}
	_, router := NewWebApp(application, authProvider, testSigningKey, t.TempDir(), "test-version", "", testStaticSubFS(t), false)
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// GET / should redirect to /setup
	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	location := resp.Header.Get("Location")
	if location != "/setup" {
		t.Errorf("Location = %q, want %q", location, "/setup")
	}

	// GET /login should also redirect to /setup
	resp, err = client.Get(ts.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("login redirect: status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}

	// GET /setup should work (200)
	resp, err = client.Get(ts.URL + "/setup")
	if err != nil {
		t.Fatalf("GET /setup: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("setup page: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestSSEHandlerSetsCorrectHeaders(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/events", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}

	cc := resp.Header.Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", cc, "no-cache")
	}
}

// responseWriterNoFlusher wraps an http.ResponseWriter without implementing http.Flusher.
type responseWriterNoFlusher struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (rw *responseWriterNoFlusher) WriteHeader(code int) {
	rw.statusCode = code
}

func (rw *responseWriterNoFlusher) Write(b []byte) (int, error) {
	return rw.body.Write(b)
}

func TestSSEReturns500WithoutFlusher(t *testing.T) {
	// We need to test the SSE handler directly since httptest.Server always provides a Flusher.
	application := newTestApp(t)
	defer func() {
		application.Stop()
		application.Wait()
	}()
	authProvider := &mock.MockAuthProvider{}
	wa, _ := NewWebApp(application, authProvider, testSigningKey, t.TempDir(), "test-version", "", testStaticSubFS(t), false)

	claims := auth.NewClaims("testuser", "mock-org-001")
	req := httptest.NewRequest("GET", "/events", nil)
	rw := &responseWriterNoFlusher{ResponseWriter: httptest.NewRecorder()}

	wa.handleEvents(rw, req, &claims)

	if rw.statusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rw.statusCode, http.StatusInternalServerError)
	}
}

func TestUploadAcceptsValidCSV(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("files", "test-data.csv")
	if err != nil {
		t.Fatalf("creating form file: %v", err)
	}
	_, _ = part.Write([]byte("header1,header2\nval1,val2\n"))
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("HX-Request", "true")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /upload: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "test-data.csv") {
		t.Errorf("response body does not mention uploaded file: %s", string(body))
	}
}

func TestFailureDetailsReturnsHTML(t *testing.T) {
	// Create an app with a stub that returns failure details
	builder := func(a *app.Application) (app.Stoppable, error) {
		return &stubRunningApp{
			stopCh: make(chan struct{}),
			finishedDetails: &app.CSVFinishedFile{
				InFile: app.FileMetadata{
					ID:               "test-record-1",
					OriginalFilename: "bad-file.csv",
					UploadedBy:       "alice",
				},
				CSVType:       csv.CSVBets,
				Success:       false,
				FailurePhase:  app.FailurePhaseProcessing,
				FailureReason: "invalid column mapping",
			},
		}, nil
	}
	application := app.NewApplication(builder)
	defer func() {
		application.Stop()
		application.Wait()
	}()

	authProvider := &mock.MockAuthProvider{}
	_, router := NewWebApp(application, authProvider, testSigningKey, t.TempDir(), "test-version", "", testStaticSubFS(t), false)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Login
	formData := "username=testuser&password=testpass"
	loginReq, _ := http.NewRequest("POST", ts.URL+"/login", strings.NewReader(formData))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginReq.Header.Set("HX-Request", "true")
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	loginResp.Body.Close()
	cookies := loginResp.Cookies()

	req, _ := http.NewRequest("GET", ts.URL+"/failure-details/test-record-1", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /failure-details: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html*", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "bad-file.csv") {
		t.Errorf("response does not contain filename: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "invalid column mapping") {
		t.Errorf("response does not contain failure reason: %s", bodyStr)
	}
}

// newTestWebAppWithStub creates a WebApp and returns the stub for direct inspection.
func newTestWebAppWithStub(t *testing.T) (*httptest.Server, *stubRunningApp, func()) {
	t.Helper()
	stub := &stubRunningApp{stopCh: make(chan struct{})}
	builder := func(a *app.Application) (app.Stoppable, error) {
		return stub, nil
	}
	application := app.NewApplication(builder)
	authProvider := &mock.MockAuthProvider{}
	_, router := NewWebApp(application, authProvider, testSigningKey, t.TempDir(), "test-version", "", testStaticSubFS(t), false)
	ts := httptest.NewServer(router)
	cleanup := func() {
		ts.Close()
		application.Stop()
		application.Wait()
	}
	return ts, stub, cleanup
}

func TestSSESendsInitialStateOnSubscribe(t *testing.T) {
	ts, _, cleanup := newTestWebAppWithStub(t)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/events", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Read SSE events; we expect 4 named events: queued, processing, uploading, recently-finished
	expectedEvents := map[string]bool{
		"queued":            false,
		"processing":       false,
		"uploading":        false,
		"recently-finished": false,
	}

	buf := make([]byte, 16384)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "event: ") {
			eventName := strings.TrimPrefix(line, "event: ")
			eventName = strings.TrimSpace(eventName)
			if _, ok := expectedEvents[eventName]; ok {
				expectedEvents[eventName] = true
			}
		}
	}

	for name, found := range expectedEvents {
		if !found {
			t.Errorf("SSE event %q was not received in initial state", name)
		}
	}
}

func TestSSEDisconnectTriggersUnsubscribe(t *testing.T) {
	ts, stub, cleanup := newTestWebAppWithStub(t)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/events", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}

	// Read at least one event to confirm connection is established
	buf := make([]byte, 16384)
	_, _ = resp.Body.Read(buf)

	// Close the response body to simulate client disconnect
	resp.Body.Close()

	// Give the server a moment to detect the disconnect and call Unsubscribe
	time.Sleep(100 * time.Millisecond)

	stub.unsubMu.Lock()
	unsub := stub.unsubscribed
	unsubID := stub.unsubscribedID
	stub.unsubMu.Unlock()

	if !unsub {
		t.Error("Unsubscribe was not called after client disconnect")
	}
	if unsubID != "test-sub" {
		t.Errorf("Unsubscribe ID = %q, want %q", unsubID, "test-sub")
	}
}

// --- Spec 14 Tests ---

// newTestWebAppWithConfiguredStub creates a WebApp and returns the stub for configuration.
func newTestWebAppWithConfiguredStub(t *testing.T, stub *stubRunningApp) (*httptest.Server, func()) {
	t.Helper()
	builder := func(a *app.Application) (app.Stoppable, error) {
		return stub, nil
	}
	application := app.NewApplication(builder)
	authProvider := &mock.MockAuthProvider{}
	_, router := NewWebApp(application, authProvider, testSigningKey, t.TempDir(), "test-version", "", testStaticSubFS(t), false)
	ts := httptest.NewServer(router)
	cleanup := func() {
		ts.Close()
		application.Stop()
		application.Wait()
	}
	return ts, cleanup
}

// testFinishedFiles returns a set of test finished files.
func testFinishedFiles() []app.CSVFinishedFile {
	now := time.Now()
	return []app.CSVFinishedFile{
		{
			InFile: app.FileMetadata{
				ID:               "file-1",
				OriginalFilename: "test-players.csv",
				UploadedBy:       "alice",
				UploadedAt:       now.Add(-2 * time.Hour),
			},
			CSVType:              csv.CSVPlayers,
			ProcessingStartedAt:  now.Add(-2 * time.Hour),
			ProcessingFinishedAt: now.Add(-1 * time.Hour),
			Success:              true,
		},
		{
			InFile: app.FileMetadata{
				ID:               "file-2",
				OriginalFilename: "bad-bets.csv",
				UploadedBy:       "bob",
				UploadedAt:       now.Add(-3 * time.Hour),
			},
			CSVType:              csv.CSVBets,
			ProcessingStartedAt:  now.Add(-3 * time.Hour),
			ProcessingFinishedAt: now.Add(-2 * time.Hour),
			Success:              false,
			FailurePhase:         app.FailurePhaseProcessing,
			FailureReason:        "invalid column mapping",
		},
		{
			InFile: app.FileMetadata{
				ID:               "file-3",
				OriginalFilename: "test-casino.csv",
				UploadedBy:       "alice",
				UploadedAt:       now.Add(-4 * time.Hour),
			},
			CSVType:              csv.CSVCasino,
			ProcessingStartedAt:  now.Add(-4 * time.Hour),
			ProcessingFinishedAt: now.Add(-3 * time.Hour),
			Success:              true,
		},
	}
}

var noRedirectClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func TestArchiveShowsAllFilesInitially(t *testing.T) {
	stub := &stubRunningApp{
		stopCh:        make(chan struct{}),
		finishedFiles: testFinishedFiles(),
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/archived", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("GET /archived: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Should contain all three files
	for _, name := range []string{"test-players.csv", "bad-bets.csv", "test-casino.csv"} {
		if !strings.Contains(bodyStr, name) {
			t.Errorf("archive page missing file %q", name)
		}
	}
}

func TestArchiveFiltersByStatus(t *testing.T) {
	stub := &stubRunningApp{
		stopCh:        make(chan struct{}),
		finishedFiles: testFinishedFiles(),
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("POST", ts.URL+"/search-archived",
		strings.NewReader("status=failure"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("POST /search-archived: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "bad-bets.csv") {
		t.Error("failure filter should include bad-bets.csv")
	}
	if strings.Contains(bodyStr, "test-players.csv") {
		t.Error("failure filter should not include test-players.csv")
	}
}

func TestArchiveFiltersByType(t *testing.T) {
	stub := &stubRunningApp{
		stopCh:        make(chan struct{}),
		finishedFiles: testFinishedFiles(),
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("POST", ts.URL+"/search-archived",
		strings.NewReader("csv_type=players"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("POST /search-archived: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "test-players.csv") {
		t.Error("type filter should include test-players.csv")
	}
	if strings.Contains(bodyStr, "bad-bets.csv") {
		t.Error("type filter should not include bad-bets.csv")
	}
}

func TestArchiveTextSearch(t *testing.T) {
	stub := &stubRunningApp{
		stopCh:        make(chan struct{}),
		finishedFiles: testFinishedFiles(),
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("POST", ts.URL+"/search-archived",
		strings.NewReader("search=test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("POST /search-archived: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "test-players.csv") {
		t.Error("text search should include test-players.csv")
	}
	if !strings.Contains(bodyStr, "test-casino.csv") {
		t.Error("text search should include test-casino.csv")
	}
	if strings.Contains(bodyStr, "bad-bets.csv") {
		t.Error("text search should not include bad-bets.csv")
	}
}

func TestArchiveDebounce(t *testing.T) {
	stub := &stubRunningApp{
		stopCh:        make(chan struct{}),
		finishedFiles: testFinishedFiles(),
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/archived", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("GET /archived: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "delay:300ms") {
		t.Error("archive page should contain hx-trigger with delay:300ms for debounce")
	}
}

func TestFailureDetailsModal(t *testing.T) {
	stub := &stubRunningApp{
		stopCh: make(chan struct{}),
		finishedDetails: &app.CSVFinishedFile{
			InFile: app.FileMetadata{
				ID:               "fail-1",
				OriginalFilename: "broken.csv",
				UploadedBy:       "charlie",
			},
			CSVType:              csv.CSVBets,
			ProcessingStartedAt:  time.Now().Add(-1 * time.Hour),
			ProcessingFinishedAt: time.Now(),
			Success:              false,
			FailurePhase:         app.FailurePhaseProcessing,
			FailureReason:        "row 42: missing required field",
		},
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/failure-details/fail-1", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("GET /failure-details: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	for _, expected := range []string{"broken.csv", "processing", "row 42: missing required field"} {
		if !strings.Contains(bodyStr, expected) {
			t.Errorf("failure details missing %q", expected)
		}
	}
}

func TestSettingsDisplaysCurrentValues(t *testing.T) {
	stub := &stubRunningApp{
		stopCh: make(chan struct{}),
		config: app.ApplicationConfig{
			OrgPlayerIDPepper: "my-secret-pepper",
			OrgPlayerIDHash:   "argon2",
			Endpoint:          "https://api.example.com",
			Environment:       "production",
			UsePlayersDB:      "true",
		},
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/settings", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("GET /settings: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	for _, expected := range []string{"my-secret-pepper", "https://api.example.com", "production", "argon2"} {
		if !strings.Contains(bodyStr, expected) {
			t.Errorf("settings page missing %q", expected)
		}
	}
}

func TestSettingsValidationError(t *testing.T) {
	stub := &stubRunningApp{
		stopCh: make(chan struct{}),
		config: app.ApplicationConfig{
			OrgPlayerIDPepper:  "valid-pepper",
			OrgPlayerIDHash:    "argon2",
			Endpoint:           "https://api.example.com",
			Environment:        "production",
			ServiceCredentials: "some-creds",
			UsePlayersDB:      "true",
		},
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	// Submit with pepper too short
	req, _ := http.NewRequest("POST", ts.URL+"/settings",
		strings.NewReader("pepper=ab&use_players_db=true"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("POST /settings: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "at least 5 characters") {
		t.Error("settings should show pepper validation error")
	}
}

func TestSettingsSaveSuccess(t *testing.T) {
	stub := &stubRunningApp{
		stopCh: make(chan struct{}),
		config: app.ApplicationConfig{
			OrgPlayerIDPepper:  "old-pepper",
			OrgPlayerIDHash:    "argon2",
			Endpoint:           "https://api.example.com",
			Environment:        "production",
			ServiceCredentials: "some-creds",
			UsePlayersDB:      "true",
		},
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("POST", ts.URL+"/settings",
		strings.NewReader("pepper=new-valid-pepper&use_players_db=false"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("POST /settings: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "Settings saved successfully") {
		t.Error("settings should show success message")
	}

	if stub.updatedConfig == nil {
		t.Fatal("UpdateConfig was not called")
	}
	if stub.updatedConfig.OrgPlayerIDPepper != "new-valid-pepper" {
		t.Errorf("pepper = %q, want %q", stub.updatedConfig.OrgPlayerIDPepper, "new-valid-pepper")
	}
	if stub.updatedConfig.UsePlayersDB != "false" {
		t.Errorf("use_players_db = %q, want %q", stub.updatedConfig.UsePlayersDB, "false")
	}
}

func TestRegistrationCode(t *testing.T) {
	stub := &stubRunningApp{
		stopCh: make(chan struct{}),
		config: app.ApplicationConfig{
			Endpoint: "https://api.example.com",
		},
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("POST", ts.URL+"/settings/registration",
		strings.NewReader("registration_code=test-code-123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("POST /settings/registration: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "accepted") {
		t.Errorf("registration code response should contain 'accepted', got: %s", bodyStr)
	}
}

func TestPlayersDBEnabled(t *testing.T) {
	now := time.Now()
	stub := &stubRunningApp{
		stopCh: make(chan struct{}),
		runningState: &app.RunningState{
			Started: now,
			PlayersDB: app.PlayersDBState{
				Enabled:     true,
				PlayerCount: 1500,
				LastUpdated: &now,
			},
		},
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/players-db", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("GET /players-db: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "1500") {
		t.Error("players DB page should show player count")
	}
	if !strings.Contains(bodyStr, "Download") {
		t.Error("players DB page should have download button")
	}
}

func TestPlayersDBDisabled(t *testing.T) {
	stub := &stubRunningApp{
		stopCh: make(chan struct{}),
		runningState: &app.RunningState{
			Started: time.Now(),
			PlayersDB: app.PlayersDBState{
				Enabled: false,
			},
		},
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/players-db", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("GET /players-db: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "disabled") {
		t.Error("players DB page should show disabled message")
	}
	if !strings.Contains(bodyStr, "/settings") {
		t.Error("players DB page should link to settings")
	}
}

func TestPlayersDBDownload(t *testing.T) {
	stub := &stubRunningApp{
		stopCh:          make(chan struct{}),
		downloadContent: "player-data-here",
		config: app.ApplicationConfig{
			OrgPlayerIDHash:   "argon2",
			OrgPlayerIDPepper: "test-pepper",
		},
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/download-players-db", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("GET /download-players-db: %v", err)
	}
	defer resp.Body.Close()

	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "players.db") {
		t.Errorf("Content-Disposition = %q, want to contain 'players.db'", cd)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "player-data-here" {
		t.Errorf("download body = %q, want %q", string(body), "player-data-here")
	}
}

func TestLoginRendersMFAConditionally(t *testing.T) {
	stub := &stubRunningApp{
		stopCh:      make(chan struct{}),
		mfaRequired: true,
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "mfa_token") {
		t.Error("login page should show MFA field when MFARequired is true")
	}

	// Now test without MFA
	stub2 := &stubRunningApp{
		stopCh:      make(chan struct{}),
		mfaRequired: false,
	}
	ts2, cleanup2 := newTestWebAppWithConfiguredStub(t, stub2)
	defer cleanup2()

	resp2, err := http.Get(ts2.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	bodyStr2 := string(body2)

	if strings.Contains(bodyStr2, "mfa_token") {
		t.Error("login page should NOT show MFA field when MFARequired is false")
	}
}

func TestLoginSuccess(t *testing.T) {
	stub := &stubRunningApp{
		stopCh: make(chan struct{}),
	}
	ts, cleanup := newTestWebAppWithConfiguredStub(t, stub)
	defer cleanup()

	formData := "username=testuser&password=testpass"
	req, _ := http.NewRequest("POST", ts.URL+"/login", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	resp.Body.Close()

	// htmx login should return HX-Redirect header
	hxRedirect := resp.Header.Get("HX-Redirect")
	if hxRedirect != "/" {
		t.Errorf("HX-Redirect = %q, want %q", hxRedirect, "/")
	}

	// Should also set session cookies
	var hasSession bool
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			hasSession = true
		}
	}
	if !hasSession {
		t.Error("login success should set session cookie")
	}
}

func TestLoginFailure(t *testing.T) {
	// Create a custom auth provider that rejects logins
	stub := &stubRunningApp{
		stopCh: make(chan struct{}),
	}
	builder := func(a *app.Application) (app.Stoppable, error) {
		return stub, nil
	}
	application := app.NewApplication(builder)
	defer func() {
		application.Stop()
		application.Wait()
	}()

	failAuthProvider := &failingAuthProvider{}
	_, router := NewWebApp(application, failAuthProvider, testSigningKey, t.TempDir(), "test-version", "", testStaticSubFS(t), false)
	ts := httptest.NewServer(router)
	defer ts.Close()

	formData := "username=testuser&password=wrongpass"
	req, _ := http.NewRequest("POST", ts.URL+"/login", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Error message should be present
	if !strings.Contains(bodyStr, "Invalid credentials") {
		t.Error("login failure should show error message")
	}
	// Username should be preserved
	if !strings.Contains(bodyStr, "testuser") {
		t.Error("login failure should preserve username")
	}
	// Password should NOT be in the response
	if strings.Contains(bodyStr, "wrongpass") {
		t.Error("login failure should not echo password")
	}
}

// failingAuthProvider always rejects login attempts.
type failingAuthProvider struct{}

func (f *failingAuthProvider) Login(username, password, mfaToken string) (app.SessionToken, error) {
	return app.SessionToken{}, fmt.Errorf("invalid credentials")
}
func (f *failingAuthProvider) ServiceAccountLogin(serviceCode string) (app.SessionToken, error) {
	return app.SessionToken{}, fmt.Errorf("not implemented")
}
func (f *failingAuthProvider) ConsumeRegistrationCode(endpoint, code string) (app.APIClient, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *failingAuthProvider) MFARequired() bool { return false }

func TestLogoutClearsCookies(t *testing.T) {
	ts, cleanup := newTestWebApp(t)
	defer cleanup()

	cookies := loginAndGetCookies(t, ts)

	req, _ := http.NewRequest("POST", ts.URL+"/logout", nil)
	req.Header.Set("HX-Request", "true")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("POST /logout: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}

	location := resp.Header.Get("Location")
	if location != "/login" {
		t.Errorf("Location = %q, want %q", location, "/login")
	}

	// Check that session cookies are cleared (MaxAge < 0)
	for _, c := range resp.Cookies() {
		if c.Name == "session" || c.Name == "session-expires" {
			if c.MaxAge >= 0 && c.Value != "" {
				t.Errorf("cookie %q should be cleared (MaxAge=%d, Value=%q)", c.Name, c.MaxAge, c.Value)
			}
		}
	}
}
