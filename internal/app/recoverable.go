package app

// RecoverableError is an error that carries a next state builder for retry.
// The Application actor detects this via errors.As and attempts one retry
// with the provided builder. If that also fails, ErrorApp is installed.
type RecoverableError struct {
	Err         error
	nextBuilder func(app *Application) (Stoppable, error)
}

// Error satisfies the error interface.
func (e *RecoverableError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *RecoverableError) Unwrap() error {
	return e.Err
}

// NextBuilder returns the state builder to use for the retry attempt.
func (e *RecoverableError) NextBuilder() func(app *Application) (Stoppable, error) {
	return e.nextBuilder
}

// NewRecoverableError creates a RecoverableError with the given error and retry builder.
func NewRecoverableError(err error, nextBuilder func(app *Application) (Stoppable, error)) *RecoverableError {
	return &RecoverableError{
		Err:         err,
		nextBuilder: nextBuilder,
	}
}
