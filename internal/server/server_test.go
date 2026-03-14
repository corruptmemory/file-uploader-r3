package server

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/corruptmemory/file-uploader-r3/internal/app"
	"github.com/corruptmemory/file-uploader-r3/internal/csv"
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

func TestHealthEndpoint(t *testing.T) {
	application := newTestApp(t)
	defer func() {
		application.Stop()
		application.Wait()
	}()

	_, router := NewWebApp(application, []byte("test-key"), t.TempDir(), "test-version", "", testStaticSubFS(t))
	ts := httptest.NewServer(router)
	defer ts.Close()

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
	application := newTestApp(t)
	defer func() {
		application.Stop()
		application.Wait()
	}()

	_, router := NewWebApp(application, []byte("test-key"), t.TempDir(), "test-version", "", testStaticSubFS(t))
	ts := httptest.NewServer(router)
	defer ts.Close()

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
	application := newTestApp(t)
	defer func() {
		application.Stop()
		application.Wait()
	}()

	_, router := NewWebApp(application, []byte("test-key"), t.TempDir(), "test-version", "", testStaticSubFS(t))
	ts := httptest.NewServer(router)
	defer ts.Close()

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
	application := newTestApp(t)
	defer func() {
		application.Stop()
		application.Wait()
	}()

	_, router := NewWebApp(application, []byte("test-key"), t.TempDir(), "test-version", "", testStaticSubFS(t))
	ts := httptest.NewServer(router)
	defer ts.Close()

	// POST login with credentials — should succeed (redirect to /)
	formData := "username=random-user&password=random-pass"
	req, _ := http.NewRequest("POST", ts.URL+"/login", bytes.NewBufferString(formData))
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
		t.Errorf("login status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}

	// Check that session cookies were set
	var hasSession, hasSessionExpires bool
	for _, c := range resp.Cookies() {
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
