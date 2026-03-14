package setup

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/corruptmemory/file-uploader-r3/internal/app"
	"github.com/corruptmemory/file-uploader-r3/internal/csv"
)

// --- Test helpers ---

// mockAuthProvider accepts any credentials.
type mockAuthProvider struct {
	failRegistration bool
	failEndpoint     bool
}

func (m *mockAuthProvider) Login(username, password, mfaToken string) (app.SessionToken, error) {
	return app.SessionToken{Username: username, OrgID: "test-org"}, nil
}

func (m *mockAuthProvider) ServiceAccountLogin(serviceCode string) (app.SessionToken, error) {
	return app.SessionToken{Username: "service", OrgID: "test-org"}, nil
}

func (m *mockAuthProvider) ConsumeRegistrationCode(endpoint, code string) (app.APIClient, error) {
	if m.failRegistration {
		return nil, fmt.Errorf("registration failed")
	}
	return &mockAPIClient{failEndpoint: m.failEndpoint}, nil
}

func (m *mockAuthProvider) MFARequired() bool { return false }

type mockAPIClient struct {
	failEndpoint bool
}

func (m *mockAPIClient) TestEndpoint() error {
	if m.failEndpoint {
		return fmt.Errorf("endpoint unreachable")
	}
	return nil
}

func (m *mockAPIClient) GetConfig(logFunc func(string, ...any)) (app.RemoteConfig, error) {
	return app.RemoteConfig{OperatorID: "TEST-OP"}, nil
}

func (m *mockAPIClient) UploadFile(_ csv.CSVType, _ int64, _ io.ReadCloser, _ func(string, ...any)) error {
	return nil
}

// noopConfigWriter is a config writer that does nothing.
func noopConfigWriter(ac app.ApplicationConfig) error {
	return nil
}

// placeholderRunningApp satisfies RunningApp for state transition tests.
type placeholderRunningApp struct {
	stopCh chan struct{}
}

func newPlaceholderRunningApp() *placeholderRunningApp {
	return &placeholderRunningApp{stopCh: make(chan struct{})}
}

func (p *placeholderRunningApp) Stop() {
	select {
	case <-p.stopCh:
	default:
		close(p.stopCh)
	}
}
func (p *placeholderRunningApp) Wait()                                      { <-p.stopCh }
func (p *placeholderRunningApp) Subscribe() (*app.EventSubscription, error) { return nil, nil }
func (p *placeholderRunningApp) Unsubscribe(id string) error                { return nil }
func (p *placeholderRunningApp) ProcessUploadedCSVFile(uploadedBy, originalFilename, localFilePath string) error {
	return nil
}
func (p *placeholderRunningApp) GetFinishedDetails(recordID string) (*app.CSVFinishedFile, error) {
	return nil, nil
}
func (p *placeholderRunningApp) GetState() (*app.RunningState, error) { return nil, nil }
func (p *placeholderRunningApp) SearchFinished(status app.FinishedStatus, csvTypes []csv.CSVType, search string) ([]app.CSVFinishedFile, error) {
	return nil, nil
}
func (p *placeholderRunningApp) GetConfig() (app.ApplicationConfig, error) {
	return app.ApplicationConfig{}, nil
}
func (p *placeholderRunningApp) MFARequired() (bool, error) { return false, nil }
func (p *placeholderRunningApp) UpdateConfig(config app.ApplicationConfig) error {
	return nil
}
func (p *placeholderRunningApp) DownloadPlayersDB(orgPlayerHash, orgPlayerIDPepper string, response http.ResponseWriter) error {
	return nil
}

var _ app.RunningApp = (*placeholderRunningApp)(nil)

// newTestSetupApp creates a SetupApp for testing with the given options.
func newTestSetupApp(t *testing.T, opts ...func(*testSetupOpts)) (*SetupApp, *app.Application) {
	t.Helper()
	o := &testSetupOpts{
		authProvider: &mockAuthProvider{},
		configWriter: noopConfigWriter,
		reason:       "",
		existingCfg:  app.ApplicationConfig{},
	}
	for _, opt := range opts {
		opt(o)
	}

	var application *app.Application
	runningAppBuilder := func(a *app.Application) (app.Stoppable, error) {
		return newPlaceholderRunningApp(), nil
	}

	// Create the application with setup as initial state
	initialBuilder := func(a *app.Application) (app.Stoppable, error) {
		return NewSetupApp(a, o.existingCfg, o.authProvider, o.configWriter, runningAppBuilder, o.reason), nil
	}
	application = app.NewApplication(initialBuilder)
	t.Cleanup(func() {
		application.Stop()
		application.Wait()
	})

	// Get the SetupApp from the application
	state, err := application.GetState()
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	sa, ok := state.(*SetupApp)
	if !ok {
		t.Fatalf("state is %T, want *SetupApp", state)
	}

	return sa, application
}

type testSetupOpts struct {
	authProvider app.AuthProvider
	configWriter func(app.ApplicationConfig) error
	reason       string
	existingCfg  app.ApplicationConfig
}

func withAuthProvider(ap app.AuthProvider) func(*testSetupOpts) {
	return func(o *testSetupOpts) { o.authProvider = ap }
}

func withConfigWriter(cw func(app.ApplicationConfig) error) func(*testSetupOpts) {
	return func(o *testSetupOpts) { o.configWriter = cw }
}

func withReason(r string) func(*testSetupOpts) {
	return func(o *testSetupOpts) { o.reason = r }
}

func withExistingConfig(cfg app.ApplicationConfig) func(*testSetupOpts) {
	return func(o *testSetupOpts) { o.existingCfg = cfg }
}

// waitForRunningApp polls the application state until it transitions to RunningApp
// or the timeout expires.
func waitForRunningApp(t *testing.T, application *app.Application, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, err := application.GetState()
		if err != nil {
			t.Fatalf("GetState: %v", err)
		}
		if _, ok := state.(app.RunningApp); ok {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for RunningApp state transition")
}

// --- Tests ---

func TestFullWizardFlow(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	var writtenConfig app.ApplicationConfig
	configWriter := func(ac app.ApplicationConfig) error {
		writtenConfig = ac
		return os.WriteFile(configPath, []byte("written"), 0600)
	}

	sa, application := newTestSetupApp(t,
		withConfigWriter(configWriter),
		withExistingConfig(app.ApplicationConfig{
			PlayersDBWorkDir: "/tmp/pdb",
			CSVUploadDir:     "/tmp/upload",
			CSVProcessingDir: "/tmp/processing",
			CSVUploadingDir:  "/tmp/uploading",
			CSVArchiveDir:    "/tmp/archive",
		}),
	)

	// Step 0: Welcome → Step 1: Endpoint
	info, err := sa.GetCurrentState()
	if err != nil {
		t.Fatalf("GetCurrentState: %v", err)
	}
	if info.CurrentStep() != app.StepWelcome {
		t.Fatalf("step = %d, want %d", info.CurrentStep(), app.StepWelcome)
	}

	info, err = sa.GetServiceEndpoint()
	if err != nil {
		t.Fatalf("GetServiceEndpoint: %v", err)
	}
	if info.CurrentStep() != app.StepEndpoint {
		t.Fatalf("step = %d, want %d", info.CurrentStep(), app.StepEndpoint)
	}

	// Step 1: Endpoint → Step 2: ServiceCredentials
	info, err = sa.SetServiceEndpoint("https://api.example.com", "production")
	if err != nil {
		t.Fatalf("SetServiceEndpoint: %v", err)
	}
	if info.CurrentStep() != app.StepServiceCredentials {
		t.Fatalf("step = %d, want %d", info.CurrentStep(), app.StepServiceCredentials)
	}

	// Step 2: ServiceCredentials → Step 3: PlayerIDHasher
	info, err = sa.UseRegistrationCode("test-code-123")
	if err != nil {
		t.Fatalf("UseRegistrationCode: %v", err)
	}
	if info.CurrentStep() != app.StepPlayerIDHasher {
		t.Fatalf("step = %d, want %d", info.CurrentStep(), app.StepPlayerIDHasher)
	}

	// Step 3: PlayerIDHasher → Step 4: UsePlayersDB
	info, err = sa.SetPlayerIDHasher("my-secret-pepper", "argon2")
	if err != nil {
		t.Fatalf("SetPlayerIDHasher: %v", err)
	}
	if info.CurrentStep() != app.StepUsePlayersDB {
		t.Fatalf("step = %d, want %d", info.CurrentStep(), app.StepUsePlayersDB)
	}

	// Step 4: UsePlayersDB → Step 5: Done
	info, err = sa.SetUsePlayerDB(true)
	if err != nil {
		t.Fatalf("SetUsePlayerDB: %v", err)
	}
	if info.CurrentStep() != app.StepDone {
		t.Fatalf("step = %d, want %d", info.CurrentStep(), app.StepDone)
	}

	// Verify config was written
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	if writtenConfig.Endpoint != "https://api.example.com" {
		t.Errorf("endpoint = %q, want %q", writtenConfig.Endpoint, "https://api.example.com")
	}
	if writtenConfig.Environment != "production" {
		t.Errorf("environment = %q, want %q", writtenConfig.Environment, "production")
	}
	if writtenConfig.OrgPlayerIDPepper != "my-secret-pepper" {
		t.Errorf("pepper = %q, want %q", writtenConfig.OrgPlayerIDPepper, "my-secret-pepper")
	}
	if writtenConfig.OrgPlayerIDHash != "argon2" {
		t.Errorf("hash = %q, want %q", writtenConfig.OrgPlayerIDHash, "argon2")
	}
	if writtenConfig.UsePlayersDB != "true" {
		t.Errorf("usePlayersDB = %q, want %q", writtenConfig.UsePlayersDB, "true")
	}

	// Wait for async state transition to RunningApp
	waitForRunningApp(t, application, 2*time.Second)
}

func TestBackwardNavigationPreservesValues(t *testing.T) {
	sa, _ := newTestSetupApp(t)

	// Advance to Endpoint
	_, err := sa.GetServiceEndpoint()
	if err != nil {
		t.Fatalf("GetServiceEndpoint: %v", err)
	}

	// Set endpoint values
	_, err = sa.SetServiceEndpoint("https://api.test.com", "staging")
	if err != nil {
		t.Fatalf("SetServiceEndpoint: %v", err)
	}

	// Go back from ServiceCredentials to Endpoint
	info, err := sa.GoBackFrom(app.StepServiceCredentials)
	if err != nil {
		t.Fatalf("GoBackFrom: %v", err)
	}
	if info.CurrentStep() != app.StepEndpoint {
		t.Fatalf("step = %d, want %d", info.CurrentStep(), app.StepEndpoint)
	}

	// Verify values are preserved
	si := info.(*StepInfo)
	if si.Endpoint != "https://api.test.com" {
		t.Errorf("endpoint = %q, want %q", si.Endpoint, "https://api.test.com")
	}
	if si.Environment != "staging" {
		t.Errorf("environment = %q, want %q", si.Environment, "staging")
	}
}

func TestStepValidationFailure(t *testing.T) {
	tests := []struct {
		name     string
		pepper   string
		wantStep app.SetupStepNumber
		wantErr  string
	}{
		{
			name:     "empty pepper",
			pepper:   "",
			wantStep: app.StepPlayerIDHasher,
			wantErr:  "Pepper is required",
		},
		{
			name:     "pepper too short",
			pepper:   "ab",
			wantStep: app.StepPlayerIDHasher,
			wantErr:  "Pepper must be at least 5 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa, _ := newTestSetupApp(t)

			// Navigate to PlayerIDHasher step
			sa.GetServiceEndpoint()
			sa.SetServiceEndpoint("https://api.test.com", "staging")
			sa.UseRegistrationCode("code-123")

			// Try invalid pepper
			info, err := sa.SetPlayerIDHasher(tt.pepper, "argon2")
			if err != nil {
				t.Fatalf("SetPlayerIDHasher: %v", err)
			}
			if info.CurrentStep() != tt.wantStep {
				t.Errorf("step = %d, want %d", info.CurrentStep(), tt.wantStep)
			}
			si := info.(*StepInfo)
			if si.ErrorMsg != tt.wantErr {
				t.Errorf("error = %q, want %q", si.ErrorMsg, tt.wantErr)
			}
		})
	}
}

func TestSkipPrevention(t *testing.T) {
	sa, _ := newTestSetupApp(t)

	// Currently at StepWelcome — try to set endpoint directly (should be ignored, stays at welcome)
	info, err := sa.SetServiceEndpoint("https://api.test.com", "staging")
	if err != nil {
		t.Fatalf("SetServiceEndpoint: %v", err)
	}
	if info.CurrentStep() != app.StepWelcome {
		t.Errorf("step = %d, want %d (skip prevented)", info.CurrentStep(), app.StepWelcome)
	}

	// Try to use registration code (should be ignored)
	info, err = sa.UseRegistrationCode("code")
	if err != nil {
		t.Fatalf("UseRegistrationCode: %v", err)
	}
	if info.CurrentStep() != app.StepWelcome {
		t.Errorf("step = %d, want %d (skip prevented)", info.CurrentStep(), app.StepWelcome)
	}

	// Try to set hasher (should be ignored)
	info, err = sa.SetPlayerIDHasher("pepper", "argon2")
	if err != nil {
		t.Fatalf("SetPlayerIDHasher: %v", err)
	}
	if info.CurrentStep() != app.StepWelcome {
		t.Errorf("step = %d, want %d (skip prevented)", info.CurrentStep(), app.StepWelcome)
	}

	// Try to set use player db (should be ignored)
	info, err = sa.SetUsePlayerDB(true)
	if err != nil {
		t.Fatalf("SetUsePlayerDB: %v", err)
	}
	if info.CurrentStep() != app.StepWelcome {
		t.Errorf("step = %d, want %d (skip prevented)", info.CurrentStep(), app.StepWelcome)
	}
}

func TestRecoveryReasonDisplayed(t *testing.T) {
	sa, _ := newTestSetupApp(t, withReason("Service connection lost"))

	info, err := sa.GetCurrentState()
	if err != nil {
		t.Fatalf("GetCurrentState: %v", err)
	}
	if info.CurrentStep() != app.StepWelcome {
		t.Fatalf("step = %d, want %d", info.CurrentStep(), app.StepWelcome)
	}
	si := info.(*StepInfo)
	if si.Reason != "Service connection lost" {
		t.Errorf("reason = %q, want %q", si.Reason, "Service connection lost")
	}
}

func TestStateTransitionOnDone(t *testing.T) {
	sa, application := newTestSetupApp(t,
		withExistingConfig(app.ApplicationConfig{
			PlayersDBWorkDir: "/tmp/pdb",
			CSVUploadDir:     "/tmp/upload",
			CSVProcessingDir: "/tmp/processing",
			CSVUploadingDir:  "/tmp/uploading",
			CSVArchiveDir:    "/tmp/archive",
		}),
	)

	// Advance through all steps
	sa.GetServiceEndpoint()
	sa.SetServiceEndpoint("https://api.example.com", "production")
	sa.UseRegistrationCode("code-123")
	sa.SetPlayerIDHasher("my-pepper-value", "argon2")
	info, err := sa.SetUsePlayerDB(false)
	if err != nil {
		t.Fatalf("SetUsePlayerDB: %v", err)
	}
	if info.CurrentStep() != app.StepDone {
		t.Fatalf("step = %d, want %d", info.CurrentStep(), app.StepDone)
	}

	// Wait for async state transition to RunningApp
	waitForRunningApp(t, application, 2*time.Second)
}

func TestEndpointValidation(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		env      string
		wantErr  string
	}{
		{
			name:     "empty endpoint",
			endpoint: "",
			env:      "staging",
			wantErr:  "Endpoint URL is required",
		},
		{
			name:     "empty environment",
			endpoint: "https://api.test.com",
			env:      "",
			wantErr:  "Environment name is required",
		},
		{
			name:     "environment too short",
			endpoint: "https://api.test.com",
			env:      "ab",
			wantErr:  "Environment must be 3-15 characters",
		},
		{
			name:     "environment too long",
			endpoint: "https://api.test.com",
			env:      "1234567890123456",
			wantErr:  "Environment must be 3-15 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa, _ := newTestSetupApp(t)

			// Advance to endpoint step
			sa.GetServiceEndpoint()

			info, err := sa.SetServiceEndpoint(tt.endpoint, tt.env)
			if err != nil {
				t.Fatalf("SetServiceEndpoint: %v", err)
			}
			if info.CurrentStep() != app.StepEndpoint {
				t.Errorf("step = %d, want %d", info.CurrentStep(), app.StepEndpoint)
			}
			si := info.(*StepInfo)
			if si.ErrorMsg != tt.wantErr {
				t.Errorf("error = %q, want %q", si.ErrorMsg, tt.wantErr)
			}
		})
	}
}

func TestRegistrationCodeValidation(t *testing.T) {
	t.Run("empty code", func(t *testing.T) {
		sa, _ := newTestSetupApp(t)

		sa.GetServiceEndpoint()
		sa.SetServiceEndpoint("https://api.test.com", "staging")

		info, err := sa.UseRegistrationCode("")
		if err != nil {
			t.Fatalf("UseRegistrationCode: %v", err)
		}
		if info.CurrentStep() != app.StepServiceCredentials {
			t.Errorf("step = %d, want %d", info.CurrentStep(), app.StepServiceCredentials)
		}
		si := info.(*StepInfo)
		if si.ErrorMsg != "Registration code is required" {
			t.Errorf("error = %q, want %q", si.ErrorMsg, "Registration code is required")
		}
	})

	t.Run("registration failure", func(t *testing.T) {
		sa, _ := newTestSetupApp(t, withAuthProvider(&mockAuthProvider{failRegistration: true}))

		sa.GetServiceEndpoint()
		sa.SetServiceEndpoint("https://api.test.com", "staging")

		info, err := sa.UseRegistrationCode("bad-code")
		if err != nil {
			t.Fatalf("UseRegistrationCode: %v", err)
		}
		if info.CurrentStep() != app.StepServiceCredentials {
			t.Errorf("step = %d, want %d", info.CurrentStep(), app.StepServiceCredentials)
		}
		si := info.(*StepInfo)
		if si.ErrorMsg == "" {
			t.Error("expected error message, got empty")
		}
	})
}

func TestExistingConfigPrePopulation(t *testing.T) {
	existingCfg := app.ApplicationConfig{
		Endpoint:          "https://existing.api.com",
		Environment:       "production",
		OrgPlayerIDPepper: "existing-pepper",
		OrgPlayerIDHash:   "argon2",
	}

	sa, _ := newTestSetupApp(t, withExistingConfig(existingCfg))

	// Advance to endpoint step
	info, err := sa.GetServiceEndpoint()
	if err != nil {
		t.Fatalf("GetServiceEndpoint: %v", err)
	}

	si := info.(*StepInfo)
	if si.Endpoint != "https://existing.api.com" {
		t.Errorf("endpoint = %q, want %q", si.Endpoint, "https://existing.api.com")
	}
	if si.Environment != "production" {
		t.Errorf("environment = %q, want %q", si.Environment, "production")
	}
}

func TestGoBackFromWelcomeStaysAtWelcome(t *testing.T) {
	sa, _ := newTestSetupApp(t)

	info, err := sa.GoBackFrom(app.StepWelcome)
	if err != nil {
		t.Fatalf("GoBackFrom: %v", err)
	}
	if info.CurrentStep() != app.StepWelcome {
		t.Errorf("step = %d, want %d", info.CurrentStep(), app.StepWelcome)
	}
}

func TestGoBackFromWrongStepIgnored(t *testing.T) {
	sa, _ := newTestSetupApp(t)

	// At Welcome step, try going back from Endpoint (wrong step)
	info, err := sa.GoBackFrom(app.StepEndpoint)
	if err != nil {
		t.Fatalf("GoBackFrom: %v", err)
	}
	// Should return current state (Welcome) since we're not at Endpoint
	if info.CurrentStep() != app.StepWelcome {
		t.Errorf("step = %d, want %d", info.CurrentStep(), app.StepWelcome)
	}
}
