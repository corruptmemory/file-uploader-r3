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
	return &app.EventSubscription{ID: "test-sub", Events: ch}, nil
}
func (s *stubRunningApp) Unsubscribe(id string) error { return nil }
func (s *stubRunningApp) GetFinishedDetails(id string) (*app.CSVFinishedFile, error) {
	if s.finishedDetails != nil {
		return s.finishedDetails, nil
	}
	return nil, fmt.Errorf("not found")
}
func (s *stubRunningApp) GetState() (*app.RunningState, error) {
	return &app.RunningState{
		Started: time.Now(),
	}, nil
}
func (s *stubRunningApp) ProcessUploadedCSVFile(uploadedBy, originalFilename, localFilePath string) error {
	return nil
}
func (s *stubRunningApp) SearchFinished(status app.FinishedStatus, csvTypes []csv.CSVType, search string) ([]app.CSVFinishedFile, error) {
	return nil, nil
}
func (s *stubRunningApp) GetConfig() (app.ApplicationConfig, error) {
	return app.ApplicationConfig{}, nil
}
func (s *stubRunningApp) MFARequired() (bool, error) { return false, nil }
func (s *stubRunningApp) UpdateConfig(config app.ApplicationConfig) error {
	return nil
}
func (s *stubRunningApp) DownloadPlayersDB(orgPlayerHash, orgPlayerIDPepper string, response http.ResponseWriter) error {
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

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
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
	part, err := writer.CreateFormFile("file", "large.csv")
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
