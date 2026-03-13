package chanutil

import (
	"errors"
	"io"
	"testing"
)

// testCommand is a minimal command struct following the actor convention.
type testCommand struct {
	tag    int
	value  string
	result chan any
}

func (c testCommand) WithResult(ch chan any) testCommand {
	c.result = ch
	return c
}

// startTestActor starts a goroutine that reads commands and responds according
// to the handler. It returns the commands channel. Close done to stop the actor.
func startTestActor(handler func(testCommand)) (chan testCommand, func()) {
	commands := make(chan testCommand, 1)
	done := make(chan struct{})
	go func() {
		defer close(commands)
		for {
			select {
			case cmd, ok := <-commands:
				if !ok {
					return
				}
				handler(cmd)
			case <-done:
				return
			}
		}
	}()
	stop := func() { close(done) }
	return commands, stop
}

func TestSendReceiveMessage(t *testing.T) {
	tests := []struct {
		name    string
		handler func(testCommand)
		wantVal string
		wantErr error
	}{
		{
			name: "returns typed result",
			handler: func(cmd testCommand) {
				cmd.result <- "hello"
			},
			wantVal: "hello",
		},
		{
			name: "returns error",
			handler: func(cmd testCommand) {
				cmd.result <- errors.New("bad thing")
			},
			wantErr: errors.New("bad thing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands, stop := startTestActor(tt.handler)
			defer stop()

			val, err := SendReceiveMessage[testCommand, string](commands, testCommand{tag: 1, value: "test"})
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr.Error() {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if val != tt.wantVal {
				t.Fatalf("expected %q, got %q", tt.wantVal, val)
			}
		})
	}
}

func TestSendReceiveError(t *testing.T) {
	tests := []struct {
		name    string
		handler func(testCommand)
		wantErr error
	}{
		{
			name: "success returns nil",
			handler: func(cmd testCommand) {
				cmd.result <- nil
			},
			wantErr: nil,
		},
		{
			name: "failure returns error",
			handler: func(cmd testCommand) {
				cmd.result <- errors.New("failed")
			},
			wantErr: errors.New("failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands, stop := startTestActor(tt.handler)
			defer stop()

			err := SendReceiveError(commands, testCommand{tag: 2})
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr.Error() {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestSendReceiveErrorOrOK(t *testing.T) {
	t.Run("success via closed channel", func(t *testing.T) {
		commands, stop := startTestActor(func(cmd testCommand) {
			close(cmd.result)
		})
		defer stop()

		err := SendReceiveErrorOrOK(commands, testCommand{tag: 3})
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("failure returns error", func(t *testing.T) {
		commands, stop := startTestActor(func(cmd testCommand) {
			cmd.result <- errors.New("nope")
		})
		defer stop()

		err := SendReceiveErrorOrOK(commands, testCommand{tag: 3})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "nope" {
			t.Fatalf("expected %q, got %q", "nope", err.Error())
		}
	})
}

func TestClosedCommandsChannel(t *testing.T) {
	t.Run("SendReceiveMessage returns EOF", func(t *testing.T) {
		commands := make(chan testCommand)
		close(commands)

		_, err := SendReceiveMessage[testCommand, string](commands, testCommand{})
		if !errors.Is(err, io.EOF) {
			t.Fatalf("expected io.EOF, got %v", err)
		}
	})

	t.Run("SendReceiveError returns EOF", func(t *testing.T) {
		commands := make(chan testCommand)
		close(commands)

		err := SendReceiveError(commands, testCommand{})
		if !errors.Is(err, io.EOF) {
			t.Fatalf("expected io.EOF, got %v", err)
		}
	})

	t.Run("SendReceiveErrorOrOK returns EOF", func(t *testing.T) {
		commands := make(chan testCommand)
		close(commands)

		err := SendReceiveErrorOrOK(commands, testCommand{})
		if !errors.Is(err, io.EOF) {
			t.Fatalf("expected io.EOF, got %v", err)
		}
	})
}

func TestResultChannelBuffered(t *testing.T) {
	// Verify that the result channel created internally has capacity 1.
	// We do this by inspecting what the actor receives.
	commands := make(chan testCommand, 1)
	go func() {
		cmd := <-commands
		if cap(cmd.result) != 1 {
			// We can't call t.Fatal from a goroutine, so send the error back.
			cmd.result <- errors.New("result channel capacity is not 1")
			return
		}
		cmd.result <- "ok"
	}()

	val, err := SendReceiveMessage[testCommand, string](commands, testCommand{tag: 99})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "ok" {
		t.Fatalf("expected %q, got %q", "ok", val)
	}
}
