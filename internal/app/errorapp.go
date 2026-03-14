package app

import "sync"

// errorApp is the concrete implementation of ErrorApp.
// It is a terminal state — no recovery is possible.
type errorApp struct {
	err  error
	stop chan struct{}
	wg   sync.WaitGroup
}

// NewErrorApp creates an ErrorApp holding the given error.
func NewErrorApp(err error) ErrorApp {
	ea := &errorApp{
		err:  err,
		stop: make(chan struct{}),
	}
	ea.wg.Add(1)
	go ea.run()
	return ea
}

func (e *errorApp) run() {
	defer e.wg.Done()
	<-e.stop
}

// Stop signals the error app goroutine to exit.
func (e *errorApp) Stop() {
	select {
	case <-e.stop:
		// already stopped
	default:
		close(e.stop)
	}
}

// Wait blocks until the error app goroutine has exited.
func (e *errorApp) Wait() {
	e.wg.Wait()
}

// GetError returns the error that caused the application to enter the error state.
func (e *errorApp) GetError() error {
	return e.err
}
