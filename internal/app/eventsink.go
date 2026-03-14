package app

import "time"

// UploadingFileMetadata describes a file being uploaded to the remote API.
type UploadingFileMetadata struct {
	InFile  FileMetadata
	OutPath string
}

// UploaderProgressRecord tracks upload progress.
type UploaderProgressRecord struct {
	BytesUploaded int64
	TotalBytes    int64
	Percent       float64
}

// UploaderEventSink receives progress events during file uploading.
type UploaderEventSink interface {
	Starting(file UploadingFileMetadata)
	Progress(file UploadingFileMetadata, record UploaderProgressRecord)
	Success(file UploadingFileMetadata)
	Failure(file UploadingFileMetadata, record UploaderProgressRecord, err error)
}

// UploadResult holds the outcome of an upload attempt.
type UploadResult struct {
	StartedAt  time.Time
	FinishedAt time.Time
	Success    bool
	Err        error
}
