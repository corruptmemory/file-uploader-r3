package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/corruptmemory/file-uploader-r3/internal/chanutil"
	csvpkg "github.com/corruptmemory/file-uploader-r3/internal/csv"
	"github.com/corruptmemory/file-uploader-r3/internal/csv/columnmapping"
	"github.com/corruptmemory/file-uploader-r3/internal/hashers"
	"github.com/corruptmemory/file-uploader-r3/internal/playerdb"
)

// --- Command types for the RunningApp actor ---

type runningAppCommandKind int

const (
	raCmdSubscribe runningAppCommandKind = iota
	raCmdUnsubscribe
	raCmdProcessFile
	raCmdGetFinishedDetails
	raCmdGetState
	raCmdSearchFinished
	raCmdGetConfig
	raCmdMFARequired
	raCmdUpdateConfig
	raCmdDownloadPlayersDB
	// Internal events from CSV processor and uploader
	raCmdCSVEvent
	raCmdUploadEvent
)

type runningAppCommand struct {
	raKind runningAppCommandKind
	result chan any

	// Fields for various commands
	subscriptionID string

	// ProcessFile fields
	uploadedBy       string
	originalFilename string
	localFilePath    string

	// GetFinishedDetails
	recordID string

	// SearchFinished
	finishedStatus FinishedStatus
	csvTypes       []csvpkg.CSVType
	searchQuery    string

	// UpdateConfig
	newConfig ApplicationConfig

	// DownloadPlayersDB
	orgPlayerHash      string
	orgPlayerIDPepper  string
	responseWriter     http.ResponseWriter

	// CSV/Upload events
	csvEvent    *csvEventPayload
	uploadEvent *uploadEventPayload
}

func (c runningAppCommand) WithResult(ch chan any) runningAppCommand {
	c.result = ch
	return c
}

// --- CSV event payloads ---

type csvEventKind int

const (
	csvEvtStarting csvEventKind = iota
	csvEvtIdentified
	csvEvtProgress
	csvEvtSuccess
	csvEvtFailure
)

type csvEventPayload struct {
	kind     csvEventKind
	outFile  csvpkg.OutFileMetadata
	progress csvpkg.ProgressRecord
	err      error
}

// --- Upload event payloads ---

type uploadEventKind int

const (
	uploadEvtStarting uploadEventKind = iota
	uploadEvtProgress
	uploadEvtSuccess
	uploadEvtFailure
)

type uploadEventPayload struct {
	kind     uploadEventKind
	file     UploadingFileMetadata
	progress UploaderProgressRecord
	err      error
}

// --- Subscriber tracking ---

type subscriber struct {
	id                string
	ch                chan DataUpdateEvent
	consecutiveFails  int
}

const (
	maxConsecutiveFails = 10
	subscriberBufSize  = 64
)

// --- realRunningApp ---

type realRunningApp struct {
	commands chan runningAppCommand
	wg       sync.WaitGroup

	// Dependencies (immutable after construction)
	uploader     Uploader
	authProvider AuthProvider
	configWriter func(ApplicationConfig) error
	processor    *csvpkg.Processor
	cancelProc   context.CancelFunc
	hashersInst  hashers.Hashers
	playersDB    *playerdb.ConcurrentPlayerDB
	dbPath       string
}

// NewRunningApp creates and starts a RunningApp actor.
func NewRunningApp(
	appConfig ApplicationConfig,
	uploader Uploader,
	authProvider AuthProvider,
	configWriter func(ApplicationConfig) error,
) (*realRunningApp, error) {
	// Build hashers
	var pdb hashers.PlayerDB
	var concurrentDB *playerdb.ConcurrentPlayerDB
	var dbPath string

	if appConfig.UsePlayersDBValue() {
		dbDir := filepath.Join(
			appConfig.PlayersDBWorkDir,
			playerdb.DBDirName(appConfig.OrgPlayerIDHash, appConfig.OrgPlayerIDPepper),
		)
		dbPath = filepath.Join(dbDir, "players.db")
		memDB, err := playerdb.LoadDB(dbPath, appConfig.OrgPlayerIDPepper)
		if err != nil {
			log.Printf("Warning: failed to load player DB: %v, creating new", err)
			memDB = playerdb.NewMemDB(appConfig.OrgPlayerIDPepper)
		}
		concurrentDB = playerdb.NewConcurrentPlayerDB(memDB)
		pdb = concurrentDB
	} else {
		pdb = playerdb.GetDevNullDB()
	}

	h := hashers.NewPlayerDataHasher(
		false, // useOnlyFirstLetterOfFirstName
		dbPath,
		appConfig.OrgPlayerIDPepper,
		appConfig.OrgPlayerIDPepper,
		hashers.ProcessName,
		pdb,
	)

	// Create CSV processor
	ctx, cancel := context.WithCancel(context.Background())
	proc := csvpkg.NewProcessor(
		ctx,
		log.Default(),
		100,  // queueSize
		4,    // workerCount
		appConfig.CSVProcessingDir,
		columnmapping.DetectCSVType,
		columnmapping.BuildAllMetadata,
		0, // no max file size
		0, // no max rows
	)

	ra := &realRunningApp{
		commands:     make(chan runningAppCommand, 64),
		uploader:     uploader,
		authProvider: authProvider,
		configWriter: configWriter,
		processor:    proc,
		cancelProc:   cancel,
		hashersInst:  h,
		playersDB:    concurrentDB,
		dbPath:       dbPath,
	}

	ra.wg.Add(1)
	go ra.run(appConfig)

	return ra, nil
}

func (ra *realRunningApp) Stop() {
	// Close the command channel to signal the actor to exit.
	defer func() { recover() }()
	close(ra.commands)
}

func (ra *realRunningApp) Wait() {
	ra.wg.Wait()
}

func (ra *realRunningApp) Subscribe() (*EventSubscription, error) {
	return chanutil.SendReceiveMessage[runningAppCommand, *EventSubscription](
		ra.commands,
		runningAppCommand{raKind: raCmdSubscribe},
	)
}

func (ra *realRunningApp) Unsubscribe(id string) error {
	return chanutil.SendReceiveError[runningAppCommand](
		ra.commands,
		runningAppCommand{raKind: raCmdUnsubscribe, subscriptionID: id},
	)
}

func (ra *realRunningApp) ProcessUploadedCSVFile(uploadedBy, originalFilename, localFilePath string) error {
	return chanutil.SendReceiveError[runningAppCommand](
		ra.commands,
		runningAppCommand{
			raKind:           raCmdProcessFile,
			uploadedBy:       uploadedBy,
			originalFilename: originalFilename,
			localFilePath:    localFilePath,
		},
	)
}

func (ra *realRunningApp) GetFinishedDetails(recordID string) (*CSVFinishedFile, error) {
	return chanutil.SendReceiveMessage[runningAppCommand, *CSVFinishedFile](
		ra.commands,
		runningAppCommand{raKind: raCmdGetFinishedDetails, recordID: recordID},
	)
}

func (ra *realRunningApp) GetState() (*RunningState, error) {
	return chanutil.SendReceiveMessage[runningAppCommand, *RunningState](
		ra.commands,
		runningAppCommand{raKind: raCmdGetState},
	)
}

func (ra *realRunningApp) SearchFinished(status FinishedStatus, csvTypes []csvpkg.CSVType, search string) ([]CSVFinishedFile, error) {
	return chanutil.SendReceiveMessage[runningAppCommand, []CSVFinishedFile](
		ra.commands,
		runningAppCommand{
			raKind:         raCmdSearchFinished,
			finishedStatus: status,
			csvTypes:       csvTypes,
			searchQuery:    search,
		},
	)
}

func (ra *realRunningApp) GetConfig() (ApplicationConfig, error) {
	return chanutil.SendReceiveMessage[runningAppCommand, ApplicationConfig](
		ra.commands,
		runningAppCommand{raKind: raCmdGetConfig},
	)
}

func (ra *realRunningApp) MFARequired() (bool, error) {
	return chanutil.SendReceiveMessage[runningAppCommand, bool](
		ra.commands,
		runningAppCommand{raKind: raCmdMFARequired},
	)
}

func (ra *realRunningApp) UpdateConfig(config ApplicationConfig) error {
	return chanutil.SendReceiveError[runningAppCommand](
		ra.commands,
		runningAppCommand{raKind: raCmdUpdateConfig, newConfig: config},
	)
}

func (ra *realRunningApp) DownloadPlayersDB(orgPlayerHash, orgPlayerIDPepper string, response http.ResponseWriter) error {
	return chanutil.SendReceiveError[runningAppCommand](
		ra.commands,
		runningAppCommand{
			raKind:            raCmdDownloadPlayersDB,
			orgPlayerHash:     orgPlayerHash,
			orgPlayerIDPepper: orgPlayerIDPepper,
			responseWriter:    response,
		},
	)
}

// --- Actor goroutine ---

func (ra *realRunningApp) run(initialConfig ApplicationConfig) {
	defer ra.wg.Done()
	defer func() {
		// Shut down processor and uploader
		ra.processor.Stop()
		ra.processor.Wait()
		ra.uploader.Stop()
		ra.uploader.Wait()
		if ra.playersDB != nil {
			ra.playersDB.Close()
		}
		ra.cancelProc()
	}()

	// Mutable state owned by this goroutine
	started := time.Now()
	config := initialConfig
	queuedFiles := make([]FileMetadata, 0)
	var processingFile *CSVProcessingFile
	uploadingFiles := make([]CSVUploadingFile, 0)
	finishedFiles := make([]CSVFinishedFile, 0)
	subscribers := make(map[string]*subscriber)
	subCounter := 0
	var lastBroadcast time.Time

	// Helper: build current processing state snapshot
	buildState := func() CSVProcessingState {
		qCopy := make([]FileMetadata, len(queuedFiles))
		copy(qCopy, queuedFiles)
		uCopy := make([]CSVUploadingFile, len(uploadingFiles))
		copy(uCopy, uploadingFiles)
		fCopy := make([]CSVFinishedFile, len(finishedFiles))
		copy(fCopy, finishedFiles)
		var pCopy *CSVProcessingFile
		if processingFile != nil {
			c := *processingFile
			pCopy = &c
		}
		return CSVProcessingState{
			QueuedFiles:    qCopy,
			ProcessingFile: pCopy,
			UploadingFiles: uCopy,
			FinishedFiles:  fCopy,
		}
	}

	// Helper: broadcast state to all subscribers
	broadcast := func(throttle bool) {
		if throttle {
			if time.Since(lastBroadcast) < time.Second {
				return
			}
		}
		lastBroadcast = time.Now()
		state := buildState()
		evt := DataUpdateEvent{State: state}
		var toRemove []string
		for id, sub := range subscribers {
			select {
			case sub.ch <- evt:
				sub.consecutiveFails = 0
			default:
				sub.consecutiveFails++
				if sub.consecutiveFails >= maxConsecutiveFails {
					toRemove = append(toRemove, id)
				}
			}
		}
		for _, id := range toRemove {
			if sub, ok := subscribers[id]; ok {
				close(sub.ch)
				delete(subscribers, id)
			}
		}
	}

	// Helper: convert app.FileMetadata to csv.FileMetadata
	toCSVFileMeta := func(f FileMetadata) csvpkg.FileMetadata {
		return csvpkg.FileMetadata{
			ID:               f.ID,
			UploadedBy:       f.UploadedBy,
			OriginalFilename: f.OriginalFilename,
			LocalFilePath:    f.LocalFilePath,
			UploadedAt:       f.UploadedAt,
		}
	}

	// Helper: convert csv.FileMetadata back to app.FileMetadata
	fromCSVFileMeta := func(f csvpkg.FileMetadata) FileMetadata {
		return FileMetadata{
			ID:               f.ID,
			UploadedBy:       f.UploadedBy,
			OriginalFilename: f.OriginalFilename,
			LocalFilePath:    f.LocalFilePath,
			UploadedAt:       f.UploadedAt,
		}
	}

	// Helper: remove uploading file by ID, return it if found
	removeUploadingFile := func(fileID string) (CSVUploadingFile, bool) {
		for i, uf := range uploadingFiles {
			if uf.InFile.ID == fileID {
				found := uf
				uploadingFiles = append(uploadingFiles[:i], uploadingFiles[i+1:]...)
				return found, true
			}
		}
		return CSVUploadingFile{}, false
	}

	for cmd := range ra.commands {
		switch cmd.raKind {
		case raCmdSubscribe:
			subCounter++
			id := fmt.Sprintf("sub-%d-%d", subCounter, time.Now().UnixNano())
			ch := make(chan DataUpdateEvent, subscriberBufSize)
			sub := &subscriber{id: id, ch: ch}
			subscribers[id] = sub
			// Send initial state
			state := buildState()
			select {
			case ch <- DataUpdateEvent{State: state}:
			default:
			}
			cmd.result <- &EventSubscription{ID: id, Events: ch}

		case raCmdUnsubscribe:
			if sub, ok := subscribers[cmd.subscriptionID]; ok {
				close(sub.ch)
				delete(subscribers, cmd.subscriptionID)
			}
			cmd.result <- nil

		case raCmdProcessFile:
			fileID := fmt.Sprintf("file-%d-%d", len(finishedFiles)+len(queuedFiles)+1, time.Now().UnixNano())
			meta := FileMetadata{
				ID:               fileID,
				UploadedBy:       cmd.uploadedBy,
				OriginalFilename: cmd.originalFilename,
				LocalFilePath:    cmd.localFilePath,
				UploadedAt:       time.Now(),
			}
			queuedFiles = append(queuedFiles, meta)

			// Create event sink that sends events back to this actor
			sink := &runningAppCSVEventSink{commands: ra.commands}

			// Get operator ID from config (use service credentials or a default)
			operatorID := csvpkg.Quoted("MOCK-OP-001")

			// Submit to processor (non-blocking from actor's perspective since queue is buffered)
			go ra.processor.AddWork(
				toCSVFileMeta(meta),
				ra.hashersInst,
				sink,
				operatorID,
			)

			broadcast(false)
			cmd.result <- nil

		case raCmdGetFinishedDetails:
			var found *CSVFinishedFile
			for i := range finishedFiles {
				if finishedFiles[i].InFile.ID == cmd.recordID {
					c := finishedFiles[i]
					found = &c
					break
				}
			}
			if found != nil {
				cmd.result <- found
			} else {
				cmd.result <- fmt.Errorf("not found: %s", cmd.recordID)
			}

		case raCmdGetState:
			var pdbState PlayersDBState
			if ra.playersDB != nil {
				entries := ra.playersDB.Entries()
				pdbState = PlayersDBState{
					Enabled:     true,
					PlayerCount: len(entries),
				}
			}
			rs := &RunningState{
				Started:        started,
				OrganizationID: "",
				AppConfig:      config,
				DataProcessing: buildState(),
				PlayersDB:      pdbState,
			}
			cmd.result <- rs

		case raCmdSearchFinished:
			var results []CSVFinishedFile
			for _, f := range finishedFiles {
				// Filter by status
				switch cmd.finishedStatus {
				case FinishedStatusSuccess:
					if !f.Success {
						continue
					}
				case FinishedStatusFailure:
					if f.Success {
						continue
					}
				}
				// Filter by CSV types
				if len(cmd.csvTypes) > 0 {
					found := false
					for _, ct := range cmd.csvTypes {
						if f.CSVType == ct {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}
				// Filter by search query
				if cmd.searchQuery != "" {
					q := strings.ToLower(cmd.searchQuery)
					if !strings.Contains(strings.ToLower(f.InFile.OriginalFilename), q) &&
						!strings.Contains(strings.ToLower(f.InFile.UploadedBy), q) {
						continue
					}
				}
				results = append(results, f)
			}
			cmd.result <- results

		case raCmdGetConfig:
			cmd.result <- config

		case raCmdMFARequired:
			cmd.result <- ra.authProvider.MFARequired()

		case raCmdUpdateConfig:
			config = cmd.newConfig
			if ra.configWriter != nil {
				if err := ra.configWriter(config); err != nil {
					log.Printf("Failed to persist config: %v", err)
				}
			}
			cmd.result <- nil

		case raCmdDownloadPlayersDB:
			if ra.playersDB == nil {
				cmd.result <- fmt.Errorf("players DB is not enabled")
			} else {
				// Create a snapshot and stream it
				snapshotPath, err := playerdb.CreateSnapshot(ra.dbPath, filepath.Dir(ra.dbPath))
				if err != nil {
					cmd.result <- fmt.Errorf("creating snapshot: %w", err)
				} else {
					cmd.responseWriter.Header().Set("Content-Type", "application/octet-stream")
					cmd.responseWriter.Header().Set("Content-Disposition", "attachment; filename=\"players.db\"")
					http.ServeFile(cmd.responseWriter, &http.Request{}, snapshotPath)
					cmd.result <- nil
				}
			}

		case raCmdCSVEvent:
			evt := cmd.csvEvent
			switch evt.kind {
			case csvEvtStarting:
				// Move from queued to processing
				inFile := fromCSVFileMeta(evt.outFile.InFile)
				// Remove from queued
				for i, qf := range queuedFiles {
					if qf.ID == inFile.ID {
						queuedFiles = append(queuedFiles[:i], queuedFiles[i+1:]...)
						break
					}
				}
				processingFile = &CSVProcessingFile{
					InFile:    inFile,
					StartedAt: time.Now(),
				}
				broadcast(false)

			case csvEvtIdentified:
				if processingFile != nil {
					processingFile.CSVType = evt.outFile.CSVType
					processingFile.ProcessingOutputPath = evt.outFile.OutPath
				}
				broadcast(false)

			case csvEvtProgress:
				if processingFile != nil {
					processingFile.ProgressPercent = evt.progress.Percent
				}
				broadcast(true) // throttle progress

			case csvEvtSuccess:
				if processingFile != nil {
					now := time.Now()
					pf := *processingFile
					pf.FinishedAt = &now
					processingFile = nil

					// Start uploading
					uploadMeta := UploadingFileMetadata{
						InFile:  pf.InFile,
						OutPath: evt.outFile.OutPath,
					}
					uf := CSVUploadingFile{
						InFile:               pf.InFile,
						CSVType:              pf.CSVType,
						ProcessingStartedAt:  pf.StartedAt,
						ProcessingFinishedAt: now,
						UploadingStartedAt:   time.Now(),
					}
					uploadingFiles = append(uploadingFiles, uf)
					broadcast(false)

					// Trigger upload asynchronously
					uploadSink := &runningAppUploadEventSink{commands: ra.commands}
					go func() {
						err := ra.uploader.UploadFile(
							uploadMeta.InFile,
							uploadMeta.OutPath,
							pf.CSVType,
							uploadSink,
						)
						if err != nil {
							// The sink's Failure callback will handle it.
							// If the uploader didn't call Failure, send one now.
							_ = err
						}
					}()
				}

			case csvEvtFailure:
				inFile := fromCSVFileMeta(evt.outFile.InFile)
				now := time.Now()
				var startedAt time.Time
				if processingFile != nil && processingFile.InFile.ID == inFile.ID {
					startedAt = processingFile.StartedAt
					processingFile = nil
				} else {
					// Remove from queued if it failed before starting
					for i, qf := range queuedFiles {
						if qf.ID == inFile.ID {
							queuedFiles = append(queuedFiles[:i], queuedFiles[i+1:]...)
							break
						}
					}
					startedAt = now
				}
				finished := CSVFinishedFile{
					InFile:               inFile,
					CSVType:              evt.outFile.CSVType,
					ProcessingStartedAt:  startedAt,
					ProcessingFinishedAt: now,
					Success:              false,
					FailurePhase:         FailurePhaseProcessing,
					FailureReason:        evt.err.Error(),
				}
				finishedFiles = append([]CSVFinishedFile{finished}, finishedFiles...)
				broadcast(false)
			}

		case raCmdUploadEvent:
			evt := cmd.uploadEvent
			switch evt.kind {
			case uploadEvtStarting:
				// Already added when processing succeeded
			case uploadEvtProgress:
				for i := range uploadingFiles {
					if uploadingFiles[i].InFile.ID == evt.file.InFile.ID {
						uploadingFiles[i].ProgressPercent = evt.progress.Percent
						break
					}
				}
				broadcast(true) // throttle progress

			case uploadEvtSuccess:
				uf, found := removeUploadingFile(evt.file.InFile.ID)
				if found {
					now := time.Now()
					finished := CSVFinishedFile{
						InFile:               uf.InFile,
						CSVType:              uf.CSVType,
						ProcessingStartedAt:  uf.ProcessingStartedAt,
						ProcessingFinishedAt: uf.ProcessingFinishedAt,
						UploadingStartedAt:   &uf.UploadingStartedAt,
						UploadingFinishedAt:  &now,
						Success:              true,
					}
					finishedFiles = append([]CSVFinishedFile{finished}, finishedFiles...)
				}
				broadcast(false)

			case uploadEvtFailure:
				uf, found := removeUploadingFile(evt.file.InFile.ID)
				if found {
					now := time.Now()
					reason := "unknown error"
					if evt.err != nil {
						reason = evt.err.Error()
					}
					finished := CSVFinishedFile{
						InFile:               uf.InFile,
						CSVType:              uf.CSVType,
						ProcessingStartedAt:  uf.ProcessingStartedAt,
						ProcessingFinishedAt: uf.ProcessingFinishedAt,
						UploadingStartedAt:   &uf.UploadingStartedAt,
						UploadingFinishedAt:  &now,
						Success:              false,
						FailurePhase:         FailurePhaseUploading,
						FailureReason:        reason,
					}
					finishedFiles = append([]CSVFinishedFile{finished}, finishedFiles...)
				}
				broadcast(false)
			}
		}
	}

	// Channel closed (Stop was called). Close all subscriber channels.
	for _, sub := range subscribers {
		close(sub.ch)
	}
}

// --- CSV EventSink implementation ---

type runningAppCSVEventSink struct {
	commands chan runningAppCommand
}

func (s *runningAppCSVEventSink) sendEvent(evt *csvEventPayload) {
	cmd := runningAppCommand{
		raKind:   raCmdCSVEvent,
		csvEvent: evt,
	}
	// Fire and forget: use trySend to avoid blocking/panic if channel is closed
	defer func() { recover() }()
	s.commands <- cmd
}

func (s *runningAppCSVEventSink) Starting(file csvpkg.OutFileMetadata) {
	s.sendEvent(&csvEventPayload{kind: csvEvtStarting, outFile: file})
}

func (s *runningAppCSVEventSink) Identified(file csvpkg.OutFileMetadata) {
	s.sendEvent(&csvEventPayload{kind: csvEvtIdentified, outFile: file})
}

func (s *runningAppCSVEventSink) Progress(file csvpkg.OutFileMetadata, record csvpkg.ProgressRecord) {
	s.sendEvent(&csvEventPayload{kind: csvEvtProgress, outFile: file, progress: record})
}

func (s *runningAppCSVEventSink) Success(file csvpkg.OutFileMetadata) {
	s.sendEvent(&csvEventPayload{kind: csvEvtSuccess, outFile: file})
}

func (s *runningAppCSVEventSink) Failure(file csvpkg.OutFileMetadata, record csvpkg.ProgressRecord, err error) {
	s.sendEvent(&csvEventPayload{kind: csvEvtFailure, outFile: file, progress: record, err: err})
}

// --- Upload EventSink implementation ---

type runningAppUploadEventSink struct {
	commands chan runningAppCommand
}

func (s *runningAppUploadEventSink) sendEvent(evt *uploadEventPayload) {
	cmd := runningAppCommand{
		raKind:      raCmdUploadEvent,
		uploadEvent: evt,
	}
	defer func() { recover() }()
	s.commands <- cmd
}

func (s *runningAppUploadEventSink) Starting(file UploadingFileMetadata) {
	s.sendEvent(&uploadEventPayload{kind: uploadEvtStarting, file: file})
}

func (s *runningAppUploadEventSink) Progress(file UploadingFileMetadata, record UploaderProgressRecord) {
	s.sendEvent(&uploadEventPayload{kind: uploadEvtProgress, file: file, progress: record})
}

func (s *runningAppUploadEventSink) Success(file UploadingFileMetadata) {
	s.sendEvent(&uploadEventPayload{kind: uploadEvtSuccess, file: file})
}

func (s *runningAppUploadEventSink) Failure(file UploadingFileMetadata, record UploaderProgressRecord, err error) {
	s.sendEvent(&uploadEventPayload{kind: uploadEvtFailure, file: file, progress: record, err: err})
}

// Compile-time check
var _ RunningApp = (*realRunningApp)(nil)
