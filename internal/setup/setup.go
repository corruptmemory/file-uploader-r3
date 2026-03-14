package setup

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/corruptmemory/file-uploader-r3/internal/app"
	"github.com/corruptmemory/file-uploader-r3/internal/chanutil"
)

// validateEndpointURL validates that the endpoint URL has a proper scheme,
// a valid hostname, and does not resolve to a private/internal IP address.
func validateEndpointURL(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL must have a valid hostname")
	}

	// Resolve the hostname to IP addresses and check for private ranges
	ips, err := net.LookupHost(host)
	if err != nil {
		// If we can't resolve, allow it — the endpoint test later will catch connectivity issues
		return nil
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if isPrivateIP(ip) {
			return fmt.Errorf("URL must not point to a private or internal address")
		}
	}

	return nil
}

// isPrivateIP checks whether an IP address belongs to a private or reserved range.
func isPrivateIP(ip net.IP) bool {
	privateRanges := []struct {
		network string
	}{
		{"127.0.0.0/8"},
		{"10.0.0.0/8"},
		{"172.16.0.0/12"},
		{"192.168.0.0/16"},
		{"169.254.0.0/16"},
		{"::1/128"},
		{"fc00::/7"},
	}

	for _, r := range privateRanges {
		_, cidr, err := net.ParseCIDR(r.network)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// --- Step Info ---

// StepInfo implements app.SetupStepInfo and carries step-specific data for templates.
type StepInfo struct {
	Step         app.SetupStepNumber
	Reason       string // Welcome step
	Endpoint     string // Endpoint step
	Environment  string // Endpoint step
	Pepper       string // PlayerIDHasher step
	Hash         string // PlayerIDHasher step
	UsePlayersDB bool   // UsePlayersDB step
	ErrorMsg     string // Any step can have an error
}

func (s *StepInfo) CurrentStep() app.SetupStepNumber { return s.Step }

func (s *StepInfo) Next() app.SetupStepNumber {
	switch s.Step {
	case app.StepWelcome:
		return app.StepEndpoint
	case app.StepEndpoint:
		return app.StepServiceCredentials
	case app.StepServiceCredentials:
		return app.StepPlayerIDHasher
	case app.StepPlayerIDHasher:
		return app.StepUsePlayersDB
	case app.StepUsePlayersDB:
		return app.StepDone
	default:
		return s.Step
	}
}

func (s *StepInfo) Prev() app.SetupStepNumber {
	switch s.Step {
	case app.StepEndpoint:
		return app.StepWelcome
	case app.StepServiceCredentials:
		return app.StepEndpoint
	case app.StepPlayerIDHasher:
		return app.StepServiceCredentials
	case app.StepUsePlayersDB:
		return app.StepPlayerIDHasher
	case app.StepError:
		return app.StepWelcome
	default:
		return s.Step
	}
}

// Compile-time check.
var _ app.SetupStepInfo = (*StepInfo)(nil)

// --- Command types ---

type commandKind int

const (
	cmdGetCurrentState commandKind = iota
	cmdGoBackFrom
	cmdGetServiceEndpoint
	cmdSetServiceEndpoint
	cmdUseRegistrationCode
	cmdSetPlayerIDHasher
	cmdSetUsePlayerDB
)

type command struct {
	result chan any
	kind   commandKind

	// Payload fields
	step         app.SetupStepNumber
	endpoint     string
	env          string
	code         string
	pepper       string
	hash         string
	usePlayersDB bool
}

func (c command) WithResult(ch chan any) command {
	c.result = ch
	return c
}

// --- Step Transition ---

type stepTransition struct {
	from app.SetupStepNumber
	to   app.SetupStepNumber
}

// --- SetupApp ---

// SetupApp implements the app.SetupApp interface using the actor pattern.
type SetupApp struct {
	commands chan command
	wg       sync.WaitGroup
}

// NewSetupApp creates a new SetupApp and starts its actor goroutine.
func NewSetupApp(
	application *app.Application,
	existingConfig app.ApplicationConfig,
	authProvider app.AuthProvider,
	configWriter func(app.ApplicationConfig) error,
	runningAppBuilder app.StateBuilder,
	reason string,
) *SetupApp {
	sa := &SetupApp{
		commands: make(chan command, 8),
	}
	sa.wg.Add(1)
	go sa.run(application, existingConfig, authProvider, configWriter, runningAppBuilder, reason)
	return sa
}

// --- Public API (all via actor channel) ---

func (sa *SetupApp) GetCurrentState() (app.SetupStepInfo, error) {
	return chanutil.SendReceiveMessage[command, app.SetupStepInfo](sa.commands, command{kind: cmdGetCurrentState})
}

func (sa *SetupApp) GoBackFrom(step app.SetupStepNumber) (app.SetupStepInfo, error) {
	return chanutil.SendReceiveMessage[command, app.SetupStepInfo](sa.commands, command{kind: cmdGoBackFrom, step: step})
}

func (sa *SetupApp) GetServiceEndpoint() (app.SetupStepInfo, error) {
	return chanutil.SendReceiveMessage[command, app.SetupStepInfo](sa.commands, command{kind: cmdGetServiceEndpoint})
}

func (sa *SetupApp) SetServiceEndpoint(endpoint, env string) (app.SetupStepInfo, error) {
	return chanutil.SendReceiveMessage[command, app.SetupStepInfo](sa.commands, command{
		kind:     cmdSetServiceEndpoint,
		endpoint: endpoint,
		env:      env,
	})
}

func (sa *SetupApp) UseRegistrationCode(code string) (app.SetupStepInfo, error) {
	return chanutil.SendReceiveMessage[command, app.SetupStepInfo](sa.commands, command{
		kind: cmdUseRegistrationCode,
		code: code,
	})
}

func (sa *SetupApp) SetPlayerIDHasher(pepper, hash string) (app.SetupStepInfo, error) {
	return chanutil.SendReceiveMessage[command, app.SetupStepInfo](sa.commands, command{
		kind:   cmdSetPlayerIDHasher,
		pepper: pepper,
		hash:   hash,
	})
}

func (sa *SetupApp) SetUsePlayerDB(usePlayersDB bool) (app.SetupStepInfo, error) {
	return chanutil.SendReceiveMessage[command, app.SetupStepInfo](sa.commands, command{
		kind:         cmdSetUsePlayerDB,
		usePlayersDB: usePlayersDB,
	})
}

func (sa *SetupApp) Stop() {
	defer func() { recover() }()
	close(sa.commands)
}

func (sa *SetupApp) Wait() {
	sa.wg.Wait()
}

// Compile-time check.
var _ app.SetupApp = (*SetupApp)(nil)

// --- Actor goroutine ---

type actorState struct {
	currentStep        app.SetupStepNumber
	endpoint           string
	environment        string
	serviceCredentials string
	pepper             string
	hashAlgorithm      string
	usePlayersDB       bool
	reason             string

	application       *app.Application
	existingConfig    app.ApplicationConfig
	authProvider      app.AuthProvider
	configWriter      func(app.ApplicationConfig) error
	runningAppBuilder app.StateBuilder
}

func (sa *SetupApp) run(
	application *app.Application,
	existingConfig app.ApplicationConfig,
	authProvider app.AuthProvider,
	configWriter func(app.ApplicationConfig) error,
	runningAppBuilder app.StateBuilder,
	reason string,
) {
	defer sa.wg.Done()

	state := &actorState{
		currentStep:        app.StepWelcome,
		endpoint:           existingConfig.Endpoint,
		environment:        existingConfig.Environment,
		serviceCredentials: existingConfig.ServiceCredentials,
		pepper:             existingConfig.OrgPlayerIDPepper,
		hashAlgorithm:      existingConfig.OrgPlayerIDHash,
		usePlayersDB:       existingConfig.UsePlayersDBValue(),
		reason:             reason,
		application:        application,
		existingConfig:     existingConfig,
		authProvider:       authProvider,
		configWriter:       configWriter,
		runningAppBuilder:  runningAppBuilder,
	}

	if state.hashAlgorithm == "" {
		state.hashAlgorithm = "argon2"
	}

	// Build dispatch map for forward transitions
	forwardHandlers := map[stepTransition]func(cmd command, s *actorState) app.SetupStepInfo{
		{app.StepWelcome, app.StepEndpoint}:                  handleWelcomeToEndpoint,
		{app.StepEndpoint, app.StepServiceCredentials}:       handleEndpointToCredentials,
		{app.StepServiceCredentials, app.StepPlayerIDHasher}: handleCredentialsToHasher,
		{app.StepPlayerIDHasher, app.StepUsePlayersDB}:       handleHasherToPlayersDB,
	}

	// Build dispatch map for backward transitions
	backwardHandlers := map[stepTransition]func(s *actorState) app.SetupStepInfo{
		{app.StepEndpoint, app.StepWelcome}:                  handleBackToWelcome,
		{app.StepServiceCredentials, app.StepEndpoint}:       handleBackToEndpoint,
		{app.StepPlayerIDHasher, app.StepServiceCredentials}: handleBackToCredentials,
		{app.StepUsePlayersDB, app.StepPlayerIDHasher}:       handleBackToHasher,
		{app.StepError, app.StepWelcome}:                     handleBackToWelcome,
	}

	for cmd := range sa.commands {
		var result app.SetupStepInfo
		pendingTransition := false

		switch cmd.kind {
		case cmdGetCurrentState:
			result = state.currentStepInfo()

		case cmdGoBackFrom:
			transition := stepTransition{from: cmd.step, to: (&StepInfo{Step: cmd.step}).Prev()}
			if cmd.step != state.currentStep {
				result = state.currentStepInfo()
			} else if handler, ok := backwardHandlers[transition]; ok {
				result = handler(state)
			} else {
				result = state.currentStepInfo()
			}

		case cmdGetServiceEndpoint:
			if state.currentStep == app.StepWelcome {
				state.currentStep = app.StepEndpoint
			}
			result = &StepInfo{
				Step:        app.StepEndpoint,
				Endpoint:    state.endpoint,
				Environment: state.environment,
			}

		case cmdSetServiceEndpoint:
			if state.currentStep != app.StepEndpoint {
				result = state.currentStepInfo()
			} else if handler, ok := forwardHandlers[stepTransition{app.StepEndpoint, app.StepServiceCredentials}]; ok {
				result = handler(cmd, state)
			} else {
				result = state.currentStepInfo()
			}

		case cmdUseRegistrationCode:
			if state.currentStep != app.StepServiceCredentials {
				result = state.currentStepInfo()
			} else if handler, ok := forwardHandlers[stepTransition{app.StepServiceCredentials, app.StepPlayerIDHasher}]; ok {
				result = handler(cmd, state)
			} else {
				result = state.currentStepInfo()
			}

		case cmdSetPlayerIDHasher:
			if state.currentStep != app.StepPlayerIDHasher {
				result = state.currentStepInfo()
			} else if handler, ok := forwardHandlers[stepTransition{app.StepPlayerIDHasher, app.StepUsePlayersDB}]; ok {
				result = handler(cmd, state)
			} else {
				result = state.currentStepInfo()
			}

		case cmdSetUsePlayerDB:
			if state.currentStep != app.StepUsePlayersDB {
				result = state.currentStepInfo()
			} else {
				result = handlePlayersDBToDoneNoTransition(cmd, state)
				if result.CurrentStep() == app.StepDone {
					pendingTransition = true
				}
			}
		}

		cmd.result <- result

		// Perform pending state transition AFTER sending the result to avoid deadlock.
		// Application.SetState() will call Stop()/Wait() on this SetupApp, so we
		// must send our response first.
		if pendingTransition {
			go func() {
				if err := state.application.SetState(state.runningAppBuilder); err != nil {
					// Log but don't block — the wizard has already responded with Done.
					_ = err
				}
			}()
		}
	}
}

// --- Forward handlers ---

func handleWelcomeToEndpoint(cmd command, s *actorState) app.SetupStepInfo {
	s.currentStep = app.StepEndpoint
	return &StepInfo{
		Step:        app.StepEndpoint,
		Endpoint:    s.endpoint,
		Environment: s.environment,
	}
}

func handleEndpointToCredentials(cmd command, s *actorState) app.SetupStepInfo {
	endpoint := cmd.endpoint
	env := cmd.env

	if endpoint == "" {
		return &StepInfo{
			Step:        app.StepEndpoint,
			Endpoint:    s.endpoint,
			Environment: s.environment,
			ErrorMsg:    "Endpoint URL is required",
		}
	}

	if env == "" {
		return &StepInfo{
			Step:        app.StepEndpoint,
			Endpoint:    endpoint,
			Environment: s.environment,
			ErrorMsg:    "Environment name is required",
		}
	}

	if len(env) < 3 || len(env) > 15 {
		return &StepInfo{
			Step:        app.StepEndpoint,
			Endpoint:    endpoint,
			Environment: env,
			ErrorMsg:    "Environment must be 3-15 characters",
		}
	}

	if err := validateEndpointURL(endpoint); err != nil {
		return &StepInfo{
			Step:        app.StepEndpoint,
			Endpoint:    endpoint,
			Environment: env,
			ErrorMsg:    fmt.Sprintf("Invalid endpoint URL: %s", err.Error()),
		}
	}

	s.endpoint = endpoint
	s.environment = env
	s.currentStep = app.StepServiceCredentials
	return &StepInfo{Step: app.StepServiceCredentials}
}

func handleCredentialsToHasher(cmd command, s *actorState) app.SetupStepInfo {
	code := cmd.code
	if code == "" {
		return &StepInfo{
			Step:     app.StepServiceCredentials,
			ErrorMsg: "Registration code is required",
		}
	}

	apiClient, err := s.authProvider.ConsumeRegistrationCode(s.endpoint, code)
	if err != nil {
		log.Printf("Registration failed for endpoint %q: %v", s.endpoint, err)
		return &StepInfo{
			Step:     app.StepServiceCredentials,
			ErrorMsg: "Registration failed — please check your code and try again",
		}
	}

	// Test the endpoint
	if err := apiClient.TestEndpoint(); err != nil {
		log.Printf("Endpoint test failed for %q: %v", s.endpoint, err)
		return &StepInfo{
			Step:     app.StepServiceCredentials,
			ErrorMsg: "Could not reach the service endpoint — please verify the URL",
		}
	}

	s.serviceCredentials = code
	s.currentStep = app.StepPlayerIDHasher
	return &StepInfo{
		Step:   app.StepPlayerIDHasher,
		Pepper: s.pepper,
		Hash:   s.hashAlgorithm,
	}
}

func handleHasherToPlayersDB(cmd command, s *actorState) app.SetupStepInfo {
	pepper := cmd.pepper
	hash := cmd.hash

	if pepper == "" {
		return &StepInfo{
			Step:     app.StepPlayerIDHasher,
			Pepper:   s.pepper,
			Hash:     s.hashAlgorithm,
			ErrorMsg: "Pepper is required",
		}
	}

	if len(pepper) < 5 {
		return &StepInfo{
			Step:     app.StepPlayerIDHasher,
			Pepper:   pepper,
			Hash:     s.hashAlgorithm,
			ErrorMsg: "Pepper must be at least 5 characters",
		}
	}

	if hash == "" {
		hash = "argon2"
	}

	s.pepper = pepper
	s.hashAlgorithm = hash
	s.currentStep = app.StepUsePlayersDB
	return &StepInfo{
		Step:         app.StepUsePlayersDB,
		UsePlayersDB: s.usePlayersDB,
	}
}

// handlePlayersDBToDoneNoTransition validates and writes config, but does NOT
// call Application.SetState. The caller must do the state transition after
// sending the response to avoid deadlock.
func handlePlayersDBToDoneNoTransition(cmd command, s *actorState) app.SetupStepInfo {
	s.usePlayersDB = cmd.usePlayersDB

	// Assemble config
	usePlayersDBStr := "false"
	if s.usePlayersDB {
		usePlayersDBStr = "true"
	}

	config := s.existingConfig.
		WithEndpoint(s.endpoint).
		WithEnvironment(s.environment).
		WithServiceCredentials(s.serviceCredentials).
		WithOrgPlayerIDPepper(s.pepper).
		WithOrgPlayerIDHash(s.hashAlgorithm).
		WithUsePlayersDB(usePlayersDBStr)

	// Validate
	if err := config.ValidateSettableValues(nil); err != nil {
		log.Printf("Configuration validation failed: %v", err)
		s.currentStep = app.StepError
		return &StepInfo{
			Step:     app.StepError,
			ErrorMsg: "Configuration validation failed — please contact your administrator",
		}
	}

	// Write config
	if err := s.configWriter(config); err != nil {
		log.Printf("Failed to write config: %v", err)
		s.currentStep = app.StepError
		return &StepInfo{
			Step:     app.StepError,
			ErrorMsg: "Failed to save configuration — please contact your administrator",
		}
	}

	s.currentStep = app.StepDone
	return &StepInfo{Step: app.StepDone}
}

// --- Backward handlers ---

func handleBackToWelcome(s *actorState) app.SetupStepInfo {
	s.currentStep = app.StepWelcome
	return &StepInfo{
		Step:   app.StepWelcome,
		Reason: s.reason,
	}
}

func handleBackToEndpoint(s *actorState) app.SetupStepInfo {
	s.currentStep = app.StepEndpoint
	return &StepInfo{
		Step:        app.StepEndpoint,
		Endpoint:    s.endpoint,
		Environment: s.environment,
	}
}

func handleBackToCredentials(s *actorState) app.SetupStepInfo {
	s.currentStep = app.StepServiceCredentials
	return &StepInfo{Step: app.StepServiceCredentials}
}

func handleBackToHasher(s *actorState) app.SetupStepInfo {
	s.currentStep = app.StepPlayerIDHasher
	return &StepInfo{
		Step:   app.StepPlayerIDHasher,
		Pepper: s.pepper,
		Hash:   s.hashAlgorithm,
	}
}

// currentStepInfo returns a StepInfo for the current step with current values.
func (s *actorState) currentStepInfo() app.SetupStepInfo {
	switch s.currentStep {
	case app.StepWelcome:
		return &StepInfo{Step: app.StepWelcome, Reason: s.reason}
	case app.StepEndpoint:
		return &StepInfo{Step: app.StepEndpoint, Endpoint: s.endpoint, Environment: s.environment}
	case app.StepServiceCredentials:
		return &StepInfo{Step: app.StepServiceCredentials}
	case app.StepPlayerIDHasher:
		return &StepInfo{Step: app.StepPlayerIDHasher, Pepper: s.pepper, Hash: s.hashAlgorithm}
	case app.StepUsePlayersDB:
		return &StepInfo{Step: app.StepUsePlayersDB, UsePlayersDB: s.usePlayersDB}
	case app.StepDone:
		return &StepInfo{Step: app.StepDone}
	case app.StepError:
		return &StepInfo{Step: app.StepError, ErrorMsg: "An error occurred"}
	default:
		return &StepInfo{Step: s.currentStep}
	}
}
