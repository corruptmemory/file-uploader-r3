package app

import "time"

// EventSubscription represents an SSE subscriber's channel and unique ID.
type EventSubscription struct {
	ID     string
	Events <-chan DataUpdateEvent
}

// DataUpdateEvent is the payload sent to SSE subscribers.
type DataUpdateEvent struct {
	State CSVProcessingState
}

// CSVProcessingState is the current state of CSV file processing.
type CSVProcessingState struct {
	QueuedFiles    []FileMetadata
	ProcessingFile *CSVProcessingFile
	UploadingFiles []CSVUploadingFile
	FinishedFiles  []CSVFinishedFile
}

// PlayersDBState holds the current state of the player deduplication database.
type PlayersDBState struct {
	Enabled     bool
	PlayerCount int
	LastUpdated *time.Time
}

// RunningState is the full state snapshot returned by RunningApp.GetState().
type RunningState struct {
	Started        time.Time
	OrganizationID string
	AppConfig      ApplicationConfig
	DataProcessing CSVProcessingState
	PlayersDB      PlayersDBState
}
