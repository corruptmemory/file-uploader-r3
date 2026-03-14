package server

import (
	"bytes"
	"embed"
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
	stopCh chan struct{}
}

func (s *stubRunningApp) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

func (s *stubRunningApp) Wait() { <-s.stopCh }

func (s *stubRunningApp) Subscribe() (*app.EventSubscription, error) { return nil, nil }
func (s *stubRunningApp) Unsubscribe(id string) error                { return nil }
func (s *stubRunningApp) GetFinishedDetails(id string) (*app.CSVFinishedFile, error) {
	return nil, nil
}
func (s *stubRunningApp) GetState() (*app.RunningState, error) { return nil, nil }
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
	_, router := NewWebApp(application, authProvider, testSigningKey, t.TempDir(), "test-version", "", testStaticSubFS(t))
	ts := httptest.NewServer(router)
	cleanup := func() {
		ts.Close()
		application.Stop()
		application.Wait()
	}
	return ts, cleanup
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
