package app

import (
	"io"

	"github.com/corruptmemory/file-uploader-r3/internal/csv"
)

// AuthProvider abstracts authentication against the remote service.
type AuthProvider interface {
	Login(username, password, mfaToken string) (SessionToken, error)
	ServiceAccountLogin(serviceCode string) (SessionToken, error)
	ConsumeRegistrationCode(endpoint, code string) (APIClient, error)
	MFARequired() bool
}

// SessionToken holds the result of a successful login.
type SessionToken struct {
	Username string
	OrgID    string
}

// APIClient abstracts the remote API for file uploads and configuration.
type APIClient interface {
	TestEndpoint() error
	GetConfig(logFunc func(string, ...any)) (RemoteConfig, error)
	UploadFile(csvType csv.CSVType, fileSize int64, reader io.ReadCloser, logFunc func(string, ...any)) error
}

// RemoteConfig holds configuration retrieved from the remote API.
type RemoteConfig struct {
	OperatorID string
}

// Uploader abstracts the file upload pipeline.
type Uploader interface {
	UploadFile(inFile FileMetadata, filePath string, csvType csv.CSVType, sink UploaderEventSink) error
	Stop()
	Wait()
}
