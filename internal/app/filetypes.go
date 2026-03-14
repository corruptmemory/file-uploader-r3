package app

import (
	"time"

	"github.com/corruptmemory/file-uploader-r3/internal/csv"
)

// FileMetadata describes an uploaded file in the application layer.
type FileMetadata struct {
	ID               string
	UploadedBy       string
	OriginalFilename string
	LocalFilePath    string
	UploadedAt       time.Time
}

// CSVProcessingFile represents a file currently being processed.
type CSVProcessingFile struct {
	InFile               FileMetadata
	CSVType              csv.CSVType
	StartedAt            time.Time
	FinishedAt           *time.Time
	ProcessingOutputPath string
	ProgressPercent      float64
}

// CSVUploadingFile represents a file currently being uploaded to the remote API.
type CSVUploadingFile struct {
	InFile               FileMetadata
	CSVType              csv.CSVType
	ProcessingStartedAt  time.Time
	ProcessingFinishedAt time.Time
	UploadingStartedAt   time.Time
	UploadingFinishedAt  *time.Time
	ProgressPercent      float64
}

// CSVFinishedFile represents a file that has completed processing and/or uploading.
type CSVFinishedFile struct {
	InFile               FileMetadata
	CSVType              csv.CSVType
	ProcessingStartedAt  time.Time
	ProcessingFinishedAt time.Time
	UploadingStartedAt   *time.Time
	UploadingFinishedAt  *time.Time
	Success              bool
	FailurePhase         FailurePhase
	FailureReason        string
}

// FailurePhase indicates which phase a file failed in.
type FailurePhase string

const (
	FailurePhaseProcessing FailurePhase = "processing"
	FailurePhaseUploading  FailurePhase = "uploading"
)

// FinishedStatus is used for filtering finished files.
type FinishedStatus string

const (
	FinishedStatusAll     FinishedStatus = ""
	FinishedStatusSuccess FinishedStatus = "success"
	FinishedStatusFailure FinishedStatus = "failure"
)
