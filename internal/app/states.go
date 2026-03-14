package app

import (
	"net/http"

	"github.com/corruptmemory/file-uploader-r3/internal/csv"
)

// SetupStepNumber identifies a step in the setup wizard.
type SetupStepNumber int

const (
	StepWelcome            SetupStepNumber = 0
	StepEndpoint           SetupStepNumber = 1
	StepServiceCredentials SetupStepNumber = 2
	StepPlayerIDHasher     SetupStepNumber = 3
	StepUsePlayersDB       SetupStepNumber = 4
	StepDone               SetupStepNumber = 5
	StepError              SetupStepNumber = 6
)

// SetupStepInfo provides information about the current setup wizard step.
type SetupStepInfo interface {
	CurrentStep() SetupStepNumber
	Next() SetupStepNumber
	Prev() SetupStepNumber
}

// SetupApp is the state interface for the setup wizard.
type SetupApp interface {
	Stoppable
	GoBackFrom(step SetupStepNumber) (SetupStepInfo, error)
	GetCurrentState() (SetupStepInfo, error)
	GetServiceEndpoint() (SetupStepInfo, error)
	SetServiceEndpoint(endpoint, env string) (SetupStepInfo, error)
	UseRegistrationCode(code string) (SetupStepInfo, error)
	SetPlayerIDHasher(pepper, hash string) (SetupStepInfo, error)
	SetUsePlayerDB(usePlayersDB bool) (SetupStepInfo, error)
}

// RunningApp is the state interface for normal operation.
type RunningApp interface {
	Stoppable
	Subscribe() (*EventSubscription, error)
	Unsubscribe(id string) error
	ProcessUploadedCSVFile(uploadedBy, originalFilename, localFilePath string) error
	GetFinishedDetails(recordID string) (*CSVFinishedFile, error)
	GetState() (*RunningState, error)
	SearchFinished(status FinishedStatus, csvTypes []csv.CSVType, search string) ([]CSVFinishedFile, error)
	GetConfig() (ApplicationConfig, error)
	MFARequired() (bool, error)
	UpdateConfig(config ApplicationConfig) error
	DownloadPlayersDB(orgPlayerHash, orgPlayerIDPepper string, response http.ResponseWriter) error
}

// ErrorApp is the terminal error state.
type ErrorApp interface {
	Stoppable
	GetError() error
}
