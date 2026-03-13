package chanutil

import "io"

// WithResulter is the interface that command structs implement to attach a
// result channel. The method uses a value receiver so it returns a copy
// without mutating the original.
type WithResulter[C any] interface {
	WithResult(chan any) C
}

// SendReceiveMessage sends cmd on commands and blocks for a typed result.
// The result channel receives either a value of type K or an error.
// If the commands channel is closed, it returns io.EOF.
func SendReceiveMessage[C WithResulter[C], K any](commands chan C, cmd C) (K, error) {
	var zero K

	resultCh := make(chan any, 1)
	cmd = cmd.WithResult(resultCh)

	// Recover from panic if commands channel is closed.
	sent := trySend(commands, cmd)
	if !sent {
		return zero, io.EOF
	}

	val, ok := <-resultCh
	if !ok {
		return zero, io.EOF
	}

	switch v := val.(type) {
	case error:
		return zero, v
	default:
		return v.(K), nil
	}
}

// SendReceiveError sends cmd on commands and returns only an error.
// The actor responds with nil (success) or an error on the result channel.
// If the commands channel is closed, it returns io.EOF.
func SendReceiveError[C WithResulter[C]](commands chan C, cmd C) error {
	resultCh := make(chan any, 1)
	cmd = cmd.WithResult(resultCh)

	sent := trySend(commands, cmd)
	if !sent {
		return io.EOF
	}

	val, ok := <-resultCh
	if !ok {
		return io.EOF
	}

	if val == nil {
		return nil
	}
	return val.(error)
}

// SendReceiveErrorOrOK sends cmd on commands. On success the actor closes the
// result channel as a signal (rather than sending nil). The function detects
// the closed channel and returns nil. On failure the actor sends an error.
// If the commands channel is closed, it returns io.EOF.
func SendReceiveErrorOrOK[C WithResulter[C]](commands chan C, cmd C) error {
	resultCh := make(chan any, 1)
	cmd = cmd.WithResult(resultCh)

	sent := trySend(commands, cmd)
	if !sent {
		return io.EOF
	}

	val, ok := <-resultCh
	if !ok {
		// Channel closed without a value — success signal.
		return nil
	}

	if val == nil {
		return nil
	}
	return val.(error)
}

// trySend attempts to send cmd on ch, recovering from a panic if ch is closed.
func trySend[C any](ch chan C, cmd C) (sent bool) {
	defer func() {
		if r := recover(); r != nil {
			sent = false
		}
	}()
	ch <- cmd
	return true
}
