package mock

import (
	"testing"
)

func TestMockAuthAcceptsAnyCreds(t *testing.T) {
	auth := &MockAuthProvider{}

	tests := []struct {
		username string
		password string
		mfa      string
	}{
		{"alice", "password123", ""},
		{"bob", "s3cr3t", "123456"},
		{"", "", ""},
		{"admin", "admin", "totp"},
	}

	for _, tt := range tests {
		token, err := auth.Login(tt.username, tt.password, tt.mfa)
		if err != nil {
			t.Errorf("Login(%q, %q, %q) returned error: %v", tt.username, tt.password, tt.mfa, err)
		}
		if token.Username != tt.username {
			t.Errorf("Login(%q, ...) Username = %q, want %q", tt.username, token.Username, tt.username)
		}
		if token.OrgID == "" {
			t.Errorf("Login(%q, ...) OrgID is empty", tt.username)
		}
	}
}

func TestMockAuthMFANotRequired(t *testing.T) {
	auth := &MockAuthProvider{}
	if auth.MFARequired() {
		t.Error("MockAuthProvider.MFARequired() = true, want false")
	}
}

func TestMockAPIClientTestEndpoint(t *testing.T) {
	client := &MockRemoteAPIClient{}
	if err := client.TestEndpoint(); err != nil {
		t.Errorf("TestEndpoint() = %v, want nil", err)
	}
}

func TestMockAPIClientGetConfig(t *testing.T) {
	client := &MockRemoteAPIClient{}
	cfg, err := client.GetConfig(nil)
	if err != nil {
		t.Errorf("GetConfig() error = %v, want nil", err)
	}
	if cfg.OperatorID == "" {
		t.Error("GetConfig() returned empty OperatorID")
	}
}
