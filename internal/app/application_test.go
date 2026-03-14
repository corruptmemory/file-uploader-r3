package app

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockStoppable is a minimal Stoppable for testing. It tracks whether
// Stop and Wait were called.
type mockStoppable struct {
	stopped  bool
	waited   bool
	stopCh   chan struct{}
	wg       sync.WaitGroup
	label    string
	mu       sync.Mutex // only for reading stopped/waited from test goroutine
	stopOnce sync.Once
}

func newMockStoppable(label string) *mockStoppable {
	m := &mockStoppable{
		stopCh: make(chan struct{}),
		label:  label,
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		<-m.stopCh
	}()
	return m
}

func (m *mockStoppable) Stop() {
	m.mu.Lock()
	m.stopped = true
	m.mu.Unlock()
	m.stopOnce.Do(func() { close(m.stopCh) })
}

func (m *mockStoppable) Wait() {
	m.wg.Wait()
	m.mu.Lock()
	m.waited = true
	m.mu.Unlock()
}

func (m *mockStoppable) wasStopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}

func (m *mockStoppable) wasWaited() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.waited
}

// mockSetupApp satisfies SetupApp for testing state type assertions.
type mockSetupApp struct {
	*mockStoppable
}

func (m *mockSetupApp) GoBackFrom(step SetupStepNumber) (SetupStepInfo, error) { return nil, nil }
func (m *mockSetupApp) GetCurrentState() (SetupStepInfo, error)                { return nil, nil }
func (m *mockSetupApp) GetServiceEndpoint() (SetupStepInfo, error)             { return nil, nil }
func (m *mockSetupApp) SetServiceEndpoint(endpoint, env string) (SetupStepInfo, error) {
	return nil, nil
}
func (m *mockSetupApp) UseRegistrationCode(code string) (SetupStepInfo, error) { return nil, nil }
func (m *mockSetupApp) SetPlayerIDHasher(pepper, hash string) (SetupStepInfo, error) {
	return nil, nil
}
func (m *mockSetupApp) SetUsePlayerDB(usePlayersDB bool) (SetupStepInfo, error) { return nil, nil }

// mockRunningApp satisfies RunningApp for testing state type assertions.
// Only the Stoppable part is used; all other methods panic if called.
type mockRunningApp struct {
	*mockStoppable
}

func newMockSetupApp() *mockSetupApp {
	return &mockSetupApp{mockStoppable: newMockStoppable("setup")}
}

func newMockRunningApp() *mockRunningApp {
	return &mockRunningApp{mockStoppable: newMockStoppable("running")}
}

func TestApplicationStartsInSetupWhenConfigNeedsSetup(t *testing.T) {
	setup := newMockSetupApp()

	app := NewApplication(func(a *Application) (Stoppable, error) {
		return setup, nil
	})
	defer func() {
		app.Stop()
		app.Wait()
	}()

	state, err := app.GetState()
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}
	if _, ok := state.(SetupApp); !ok {
		t.Fatalf("expected SetupApp, got %T", state)
	}
}

func TestApplicationStartsInRunningWhenConfigComplete(t *testing.T) {
	running := newMockRunningApp()

	app := NewApplication(func(a *Application) (Stoppable, error) {
		return running, nil
	})
	defer func() {
		app.Stop()
		app.Wait()
	}()

	state, err := app.GetState()
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}
	if _, ok := state.(*mockRunningApp); !ok {
		t.Fatalf("expected *mockRunningApp, got %T", state)
	}
}

func TestSetStateStopsOldStateFirst(t *testing.T) {
	oldState := newMockStoppable("old")

	app := NewApplication(func(a *Application) (Stoppable, error) {
		return oldState, nil
	})
	defer func() {
		app.Stop()
		app.Wait()
	}()

	// Verify old state is running.
	_, err := app.GetState()
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}

	newState := newMockStoppable("new")
	stoppedBeforeNew := false

	err = app.SetState(func(a *Application) (Stoppable, error) {
		// At this point, old state should have been stopped and waited on.
		stoppedBeforeNew = oldState.wasStopped() && oldState.wasWaited()
		return newState, nil
	})
	if err != nil {
		t.Fatalf("SetState() error: %v", err)
	}

	if !stoppedBeforeNew {
		t.Fatal("old state was not stopped+waited before new state builder was called")
	}

	// Verify new state is current.
	state, err := app.GetState()
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}
	if state != newState {
		t.Fatalf("expected new state, got different state")
	}
}

func TestRecoverableErrorRetriesWithNextBuilder(t *testing.T) {
	recoveredState := newMockStoppable("recovered")
	builderCalls := 0

	app := NewApplication(func(a *Application) (Stoppable, error) {
		builderCalls++
		return nil, NewRecoverableError(
			fmt.Errorf("credentials expired"),
			func(a *Application) (Stoppable, error) {
				builderCalls++
				return recoveredState, nil
			},
		)
	})
	defer func() {
		app.Stop()
		app.Wait()
	}()

	state, err := app.GetState()
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}
	if state != recoveredState {
		t.Fatalf("expected recovered state, got %T", state)
	}
	if builderCalls != 2 {
		t.Fatalf("expected 2 builder calls (original + retry), got %d", builderCalls)
	}
}

func TestDoubleRecoveryResultsInErrorApp(t *testing.T) {
	app := NewApplication(func(a *Application) (Stoppable, error) {
		return nil, NewRecoverableError(
			fmt.Errorf("first failure"),
			func(a *Application) (Stoppable, error) {
				// Second builder also fails (non-recoverable).
				return nil, fmt.Errorf("second failure")
			},
		)
	})
	defer func() {
		app.Stop()
		app.Wait()
	}()

	state, err := app.GetState()
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}
	errState, ok := state.(ErrorApp)
	if !ok {
		t.Fatalf("expected ErrorApp, got %T", state)
	}
	if errState.GetError() == nil {
		t.Fatal("expected non-nil error from ErrorApp")
	}
	if errState.GetError().Error() != "second failure" {
		t.Fatalf("expected 'second failure', got %q", errState.GetError().Error())
	}
}

func TestGetStateReturnsCurrentState(t *testing.T) {
	ms := newMockStoppable("test")

	app := NewApplication(func(a *Application) (Stoppable, error) {
		return ms, nil
	})
	defer func() {
		app.Stop()
		app.Wait()
	}()

	state, err := app.GetState()
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}
	if state != ms {
		t.Fatal("GetState returned wrong state")
	}
}

func TestSetStateWithRecoverableError(t *testing.T) {
	initial := newMockStoppable("initial")
	recovered := newMockStoppable("recovered")

	app := NewApplication(func(a *Application) (Stoppable, error) {
		return initial, nil
	})
	defer func() {
		app.Stop()
		app.Wait()
	}()

	err := app.SetState(func(a *Application) (Stoppable, error) {
		return nil, NewRecoverableError(
			fmt.Errorf("oops"),
			func(a *Application) (Stoppable, error) {
				return recovered, nil
			},
		)
	})
	if err != nil {
		t.Fatalf("SetState() error: %v", err)
	}

	state, err := app.GetState()
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}
	if state != recovered {
		t.Fatalf("expected recovered state after SetState with RecoverableError")
	}
}

func TestSetStateDoubleRecoveryResultsInErrorApp(t *testing.T) {
	initial := newMockStoppable("initial")

	app := NewApplication(func(a *Application) (Stoppable, error) {
		return initial, nil
	})
	defer func() {
		app.Stop()
		app.Wait()
	}()

	err := app.SetState(func(a *Application) (Stoppable, error) {
		return nil, NewRecoverableError(
			fmt.Errorf("first"),
			func(a *Application) (Stoppable, error) {
				return nil, fmt.Errorf("second")
			},
		)
	})
	if err != nil {
		t.Fatalf("SetState() error: %v", err)
	}

	state, err := app.GetState()
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}
	errState, ok := state.(ErrorApp)
	if !ok {
		t.Fatalf("expected ErrorApp after double recovery failure, got %T", state)
	}
	if errState.GetError().Error() != "second" {
		t.Fatalf("expected error 'second', got %q", errState.GetError().Error())
	}
}

func TestErrorAppGetError(t *testing.T) {
	original := fmt.Errorf("something broke")
	ea := NewErrorApp(original)
	defer func() {
		ea.Stop()
		ea.Wait()
	}()

	if ea.GetError() != original {
		t.Fatalf("expected original error, got %v", ea.GetError())
	}
}

func TestErrorAppStopWait(t *testing.T) {
	ea := NewErrorApp(fmt.Errorf("test"))
	ea.Stop()

	done := make(chan struct{})
	go func() {
		ea.Wait()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("ErrorApp.Wait() did not return after Stop()")
	}
}

func TestApplicationStopCleansUpState(t *testing.T) {
	ms := newMockStoppable("cleanup")

	app := NewApplication(func(a *Application) (Stoppable, error) {
		return ms, nil
	})

	// Ensure state is running.
	_, err := app.GetState()
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}

	app.Stop()
	app.Wait()

	if !ms.wasStopped() {
		t.Fatal("state was not stopped when Application was stopped")
	}
	if !ms.wasWaited() {
		t.Fatal("state was not waited on when Application was stopped")
	}
}

func TestRecoverableErrorSatisfiesErrorInterface(t *testing.T) {
	var err error = NewRecoverableError(fmt.Errorf("test"), nil)
	if err.Error() != "test" {
		t.Fatalf("expected 'test', got %q", err.Error())
	}
}

// --- SSE subscriber tests ---
// These test the Subscribe/Unsubscribe contract for RunningApp implementations.

// mockSSERunningApp is a minimal RunningApp with SSE subscriber support.
type mockSSERunningApp struct {
	*mockStoppable
	subscribers    map[string]chan DataUpdateEvent
	subMu          sync.Mutex
	nextID         int
	failCounts     map[string]int // consecutive send failures per subscriber
	maxConsecutive int            // auto-remove threshold
}

func newMockSSERunningApp() *mockSSERunningApp {
	return &mockSSERunningApp{
		mockStoppable:  newMockStoppable("sse-running"),
		subscribers:    make(map[string]chan DataUpdateEvent),
		failCounts:     make(map[string]int),
		maxConsecutive: 10,
	}
}

func (m *mockSSERunningApp) Subscribe() (*EventSubscription, error) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	m.nextID++
	id := fmt.Sprintf("sub-%d", m.nextID)
	ch := make(chan DataUpdateEvent, 1)
	m.subscribers[id] = ch
	m.failCounts[id] = 0
	return &EventSubscription{ID: id, Events: ch}, nil
}

func (m *mockSSERunningApp) Unsubscribe(id string) error {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	if ch, ok := m.subscribers[id]; ok {
		close(ch)
		delete(m.subscribers, id)
		delete(m.failCounts, id)
	}
	return nil
}

// broadcast sends an event to all subscribers with non-blocking sends.
// Tracks consecutive failures and auto-removes after 10 consecutive drops.
func (m *mockSSERunningApp) broadcast(event DataUpdateEvent) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	for id, ch := range m.subscribers {
		select {
		case ch <- event:
			m.failCounts[id] = 0
		default:
			m.failCounts[id]++
			if m.failCounts[id] >= m.maxConsecutive {
				close(ch)
				delete(m.subscribers, id)
				delete(m.failCounts, id)
			}
		}
	}
}

func (m *mockSSERunningApp) subscriberCount() int {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	return len(m.subscribers)
}

func TestSSESubscriberReceivesUpdates(t *testing.T) {
	ra := newMockSSERunningApp()
	defer ra.Stop()

	sub, err := ra.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}
	defer ra.Unsubscribe(sub.ID)

	// Broadcast an event.
	event := DataUpdateEvent{State: CSVProcessingState{}}
	ra.broadcast(event)

	// Subscriber should receive the event.
	select {
	case got := <-sub.Events:
		_ = got // received
	case <-time.After(time.Second):
		t.Fatal("SSE subscriber did not receive update within timeout")
	}
}

func TestAutoRemoveAfter10ConsecutiveFailures(t *testing.T) {
	ra := newMockSSERunningApp()
	defer ra.Stop()

	// Subscribe but never drain the channel (buffer size 1).
	sub, err := ra.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	if ra.subscriberCount() != 1 {
		t.Fatalf("expected 1 subscriber, got %d", ra.subscriberCount())
	}

	event := DataUpdateEvent{State: CSVProcessingState{}}

	// First broadcast fills the buffer (success, no failure).
	ra.broadcast(event)

	// Now send 10 more without draining — these are consecutive failures.
	// After 10 consecutive failures the subscriber should be auto-removed.
	for i := 0; i < 10; i++ {
		ra.broadcast(event)
	}

	if ra.subscriberCount() != 0 {
		t.Fatalf("expected subscriber auto-removed after 10 consecutive failures, still have %d", ra.subscriberCount())
	}

	// Drain buffered event, then verify channel is closed.
	// There's 1 buffered event from the first successful broadcast.
	select {
	case <-sub.Events:
	default:
	}
	_, open := <-sub.Events
	if open {
		t.Fatal("expected subscriber channel to be closed after auto-remove")
	}
}

func TestInitialBuilderNonRecoverableError(t *testing.T) {
	app := NewApplication(func(a *Application) (Stoppable, error) {
		return nil, fmt.Errorf("fatal boot error")
	})
	defer func() {
		app.Stop()
		app.Wait()
	}()

	state, err := app.GetState()
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}
	errState, ok := state.(ErrorApp)
	if !ok {
		t.Fatalf("expected ErrorApp on non-recoverable initial error, got %T", state)
	}
	if errState.GetError().Error() != "fatal boot error" {
		t.Fatalf("expected 'fatal boot error', got %q", errState.GetError().Error())
	}
}
