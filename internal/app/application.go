package app

import (
	"errors"
	"sync"

	"github.com/corruptmemory/file-uploader-r3/internal/chanutil"
)

// StateBuilder is a function that creates a new Stoppable state for the Application.
type StateBuilder func(app *Application) (Stoppable, error)

// appCommand is the command type for the Application actor's channel.
type appCommand struct {
	result chan any
	kind   appCommandKind
	// payload for setState
	stateBuilder StateBuilder
}

type appCommandKind int

const (
	cmdGetState appCommandKind = iota
	cmdSetState
)

func (c appCommand) WithResult(ch chan any) appCommand {
	c.result = ch
	return c
}

// Application is the top-level orchestrator. It owns exactly one active state
// at a time and processes commands via a single goroutine (actor pattern).
type Application struct {
	commands chan appCommand
	wg       sync.WaitGroup
}

// NewApplication creates an Application and starts its run goroutine.
// The initialStateBuilder is called inside the run goroutine to create the first state.
func NewApplication(initialStateBuilder StateBuilder) *Application {
	a := &Application{
		commands: make(chan appCommand, 8),
	}
	a.wg.Add(1)
	go a.run(initialStateBuilder)
	return a
}

// GetState returns the current state. Callers type-assert to SetupApp, RunningApp, or ErrorApp.
func (a *Application) GetState() (any, error) {
	return chanutil.SendReceiveMessage[appCommand, any](a.commands, appCommand{kind: cmdGetState})
}

// SetState transitions to a new state. The current state is stopped and waited on
// before the new state is created.
func (a *Application) SetState(builder StateBuilder) error {
	return chanutil.SendReceiveError(a.commands, appCommand{
		kind:         cmdSetState,
		stateBuilder: builder,
	})
}

// Stop closes the commands channel, signaling the run goroutine to exit.
func (a *Application) Stop() {
	close(a.commands)
}

// Wait blocks until the run goroutine has exited.
func (a *Application) Wait() {
	a.wg.Wait()
}

// run is the actor goroutine. It owns the current state and processes commands.
func (a *Application) run(initialStateBuilder StateBuilder) {
	defer a.wg.Done()

	currentState := a.buildStateWithRecovery(initialStateBuilder)

	defer func() {
		if currentState != nil {
			currentState.Stop()
			currentState.Wait()
		}
	}()

	for cmd := range a.commands {
		switch cmd.kind {
		case cmdGetState:
			cmd.result <- currentState
		case cmdSetState:
			// Stop and wait on old state before creating new one.
			if currentState != nil {
				currentState.Stop()
				currentState.Wait()
			}
			currentState = a.buildStateWithRecovery(cmd.stateBuilder)
			cmd.result <- nil // success
		}
	}
}

// buildStateWithRecovery calls a state builder, handling RecoverableError with one retry.
// If both the original and retry fail, an ErrorApp is installed.
func (a *Application) buildStateWithRecovery(builder StateBuilder) Stoppable {
	state, err := builder(a)
	if err == nil {
		return state
	}

	// Check for RecoverableError — one retry level.
	var recErr *RecoverableError
	if errors.As(err, &recErr) {
		nextBuilder := recErr.NextBuilder()
		state, err = nextBuilder(a)
		if err == nil {
			return state
		}
		// Double failure — fall through to ErrorApp.
	}

	return NewErrorApp(err)
}
