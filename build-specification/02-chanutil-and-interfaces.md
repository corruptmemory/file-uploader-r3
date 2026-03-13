# 02 — Channel Utilities and Core Interfaces

**Dependencies:** 01-project-scaffold.md (project compiles).

**Produces:** `internal/chanutil/` package, `Stoppable` interface, core type aliases used by all actors.

---

## 1. Generic Channel Helpers (`internal/chanutil/`)

Reusable functions that eliminate boilerplate in the actor pattern. Every actor in the application uses these.

### Command Struct Convention

Every actor's command struct must have:

```go
type command struct {
    tag    commandTag
    // ... domain-specific fields ...
    result chan any
}

func (c command) WithResult(ch chan any) command {
    c.result = ch
    return c
}
```

The `WithResult` method returns a copy (value receiver) so the original command is not mutated.

### Functions

```go
package chanutil

// SendReceiveMessage sends a command on the channel and returns a typed result.
func SendReceiveMessage[C any, K any](commands chan C, cmd C) (K, error)

// SendReceiveError sends a command and returns only an error.
func SendReceiveError[C any](commands chan C, cmd C) error

// SendReceiveErrorOrOK sends a command. On success (nil error), closes the result channel.
func SendReceiveErrorOrOK[C any](commands chan C, cmd C) error
```

### Behavior

All three follow this pattern:

1. Create a `result` channel: `make(chan any, 1)`.
2. Call `WithResult(resultCh)` on the command (via interface or type assertion — use a `WithResulter` interface).
3. Send the command on the `commands` channel.
4. Block on the `result` channel.
5. Handle panics: if the result channel is closed without a value, return `io.EOF`.
6. Type-assert the received value.

**`SendReceiveMessage[C, K]`:** The result channel receives either an `error` or a value of type `K`. If error, return zero `K` and error. If `K`, return it with nil error.

**`SendReceiveError[C]`:** The result channel receives either `nil` (success) or an `error`.

**`SendReceiveErrorOrOK[C]`:** Same as `SendReceiveError`, but on success the actor's `run()` goroutine closes the result channel as a signal. The function detects a closed channel and returns nil.

### WithResulter Interface

```go
type WithResulter[C any] interface {
    WithResult(chan any) C
}
```

The generic functions use this interface to attach the result channel to the command.

### Panic Recovery

If the `commands` channel is closed (actor has stopped), sending on it will panic. The functions must recover from this panic and return `io.EOF`.

### Constraints

- These functions are generic — they must NOT import any application-specific types.
- Result channels are always buffered with capacity 1.
- The `WithResult` convention uses a value receiver.

---

## 2. Stoppable Interface

```go
package app // internal/app/

type Stoppable interface {
    Stop()  // signals the goroutine to shut down (non-blocking)
    Wait()  // blocks until the goroutine has fully exited
}
```

Every application state (SetupApp, RunningApp, ErrorApp) implements `Stoppable`.

---

## Tests

### chanutil Tests (`internal/chanutil/chanutil_test.go`)

Table-driven tests:

| Test | Description |
|------|-------------|
| SendReceiveMessage returns typed result | Send command, actor responds with value, verify type and content |
| SendReceiveMessage returns error | Send command, actor responds with error, verify error propagation |
| SendReceiveError success | Send command, actor responds with nil, verify nil return |
| SendReceiveError failure | Send command, actor responds with error, verify error return |
| Closed channel returns io.EOF | Close the commands channel, attempt send, verify io.EOF |
| Result channel is buffered | Verify result channel capacity is 1 |

### Test Pattern

Create a minimal test actor:

```go
type testCommand struct {
    tag    int
    value  string
    result chan any
}

func (c testCommand) WithResult(ch chan any) testCommand {
    c.result = ch
    return c
}
```

Start a goroutine that reads commands and responds, then exercise the chanutil functions against it.

## Acceptance Criteria

- [ ] `SendReceiveMessage` returns a typed result or an error
- [ ] `SendReceiveError` returns nil on success or the error on failure
- [ ] All three functions return `io.EOF` if the commands channel is closed
- [ ] Result channels are buffered with capacity 1
- [ ] The `WithResult` convention uses a value receiver to return a copy
- [ ] All tests pass
