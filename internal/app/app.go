package app

// Stoppable is implemented by every application state (SetupApp, RunningApp,
// ErrorApp) that runs a background goroutine.
type Stoppable interface {
	Stop() // signals the goroutine to shut down (non-blocking)
	Wait() // blocks until the goroutine has fully exited
}
