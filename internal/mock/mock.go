package mock

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/corruptmemory/file-uploader-r3/internal/app"
	"github.com/corruptmemory/file-uploader-r3/internal/csv"
)

// --- MockAuthProvider ---

// MockAuthProvider accepts any credentials.
type MockAuthProvider struct{}

func (m *MockAuthProvider) Login(username, password, mfaToken string) (app.SessionToken, error) {
	return app.SessionToken{
		Username: username,
		OrgID:    "mock-org-001",
	}, nil
}

func (m *MockAuthProvider) ServiceAccountLogin(serviceCode string) (app.SessionToken, error) {
	return app.SessionToken{
		Username: "service-account",
		OrgID:    "mock-org-001",
	}, nil
}

func (m *MockAuthProvider) ConsumeRegistrationCode(endpoint, code string) (app.APIClient, error) {
	return &MockRemoteAPIClient{OutputDir: ""}, nil
}

func (m *MockAuthProvider) MFARequired() bool {
	return false
}

// --- MockRemoteAPIClient ---

// MockRemoteAPIClient returns static responses and writes uploads to OutputDir.
type MockRemoteAPIClient struct {
	OutputDir string
}

func (m *MockRemoteAPIClient) TestEndpoint() error {
	return nil
}

func (m *MockRemoteAPIClient) GetConfig(logFunc func(string, ...any)) (app.RemoteConfig, error) {
	if logFunc != nil {
		logFunc("mock: returning static config")
	}
	return app.RemoteConfig{
		OperatorID: "MOCK-OP-001",
	}, nil
}

func (m *MockRemoteAPIClient) UploadFile(csvType csv.CSVType, fileSize int64, reader io.ReadCloser, logFunc func(string, ...any)) error {
	defer reader.Close()

	if m.OutputDir == "" {
		// Discard
		_, _ = io.Copy(io.Discard, reader)
		return nil
	}

	if err := os.MkdirAll(m.OutputDir, 0o750); err != nil {
		return fmt.Errorf("mock: create output dir: %w", err)
	}

	outPath := filepath.Join(m.OutputDir, fmt.Sprintf("mock-upload-%s-%d.csv", csvType.Slug(), time.Now().UnixNano()))
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("mock: create output file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, reader); err != nil {
		return fmt.Errorf("mock: write output file: %w", err)
	}

	if logFunc != nil {
		logFunc("mock: uploaded to %s", outPath)
	}
	return nil
}

// --- MockUploader ---

// MockUploaderOption configures a MockUploader.
type MockUploaderOption func(*MockUploader)

// WithFailure configures the mock to fail after the given percentage.
func WithFailure(afterPercent float64) MockUploaderOption {
	return func(m *MockUploader) {
		m.failAfterPercent = afterPercent
	}
}

// WithDelay configures the mock to wait between progress steps.
func WithDelay(d time.Duration) MockUploaderOption {
	return func(m *MockUploader) {
		m.delay = d
	}
}

// MockUploader simulates file upload with progress events.
type MockUploader struct {
	outputDir        string
	failAfterPercent float64 // 0 means no failure
	delay            time.Duration
	stopCh           chan struct{}
}

// NewMockUploader creates a MockUploader with the given options.
func NewMockUploader(outputDir string, opts ...MockUploaderOption) *MockUploader {
	m := &MockUploader{
		outputDir: outputDir,
		stopCh:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *MockUploader) UploadFile(inFile app.FileMetadata, filePath string, csvType csv.CSVType, sink app.UploaderEventSink) error {
	uploadMeta := app.UploadingFileMetadata{
		InFile:  inFile,
		OutPath: filePath,
	}

	// Get file size for progress tracking
	fi, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("mock uploader: stat file: %w", err)
	}
	totalBytes := fi.Size()

	sink.Starting(uploadMeta)

	steps := []float64{25, 50, 75, 100}
	for _, pct := range steps {
		if m.failAfterPercent > 0 && pct > m.failAfterPercent {
			sink.Failure(uploadMeta, app.UploaderProgressRecord{
				BytesUploaded: int64(float64(totalBytes) * m.failAfterPercent / 100),
				TotalBytes:    totalBytes,
				Percent:       m.failAfterPercent,
			}, fmt.Errorf("mock: simulated failure at %.0f%%", m.failAfterPercent))
			return fmt.Errorf("mock: simulated failure at %.0f%%", m.failAfterPercent)
		}

		if m.delay > 0 {
			select {
			case <-m.stopCh:
				return nil
			case <-time.After(m.delay):
			}
		}

		sink.Progress(uploadMeta, app.UploaderProgressRecord{
			BytesUploaded: int64(float64(totalBytes) * pct / 100),
			TotalBytes:    totalBytes,
			Percent:       pct,
		})
	}

	// Copy file to output dir if configured
	if m.outputDir != "" {
		if err := os.MkdirAll(m.outputDir, 0o750); err != nil {
			return fmt.Errorf("mock uploader: mkdir: %w", err)
		}
		destPath := filepath.Join(m.outputDir, filepath.Base(filePath))
		srcFile, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("mock uploader: open source: %w", err)
		}
		defer srcFile.Close()

		dstFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("mock uploader: create dest: %w", err)
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return fmt.Errorf("mock uploader: copy: %w", err)
		}
	}

	sink.Success(uploadMeta)
	return nil
}

func (m *MockUploader) Stop() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
}

func (m *MockUploader) Wait() {
	// MockUploader processes synchronously, nothing to wait for.
}

// Compile-time interface checks.
var (
	_ app.AuthProvider = (*MockAuthProvider)(nil)
	_ app.APIClient    = (*MockRemoteAPIClient)(nil)
	_ app.Uploader     = (*MockUploader)(nil)
)
