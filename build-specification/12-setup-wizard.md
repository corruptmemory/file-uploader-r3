# 12 — Setup Wizard

**Dependencies:** 10-application-state-machine.md (SetupApp interface, SetupStepNumber), 11-auth-and-server.md (route table, AuthProvider).

**Design System:** Follow the design system in `design-system/` for all visual implementation. Use `tokens.css` for colors, spacing, and fonts. Follow `patterns.md` for component patterns. Follow `layout.md` for page structure and responsive behavior.

**Produces:** SetupApp actor implementation, wizard page templates, setup route handlers.

---

## 1. SetupApp Actor Implementation

SetupApp uses the actor pattern. Its `run()` goroutine owns the current step number and all accumulated wizard state.

### Internal State

- Current step number
- Accumulated values: endpoint, environment, service credentials, pepper, hash algorithm, usePlayersDB flag
- Reference to parent Application (for SetState on completion)
- `runningAppBuilder` closure (creates RunningApp from valid config)
- Optional reason message (when recovering from RunningApp)
- Existing config values (for pre-population)

### Wizard Dispatch

Use a map keyed by `(currentStep, targetStep)` pairs → handler functions. **No nested switch blocks deeper than one level.**

```go
type stepTransition struct {
    from SetupStepNumber
    to   SetupStepNumber
}

handlers := map[stepTransition]func(cmd command) SetupStepInfo{
    {StepWelcome, StepEndpoint}:             handleWelcomeToEndpoint,
    {StepEndpoint, StepServiceCredentials}:  handleEndpointToCredentials,
    // ...
}
```

---

## 2. Wizard Steps

| Step | Name | Description |
|------|------|-------------|
| 0 | Welcome | Intro text. If recovering from RunningApp, shows reason. "Get Started" button. |
| 1 | Endpoint | Inputs: environment name, endpoint URL. Validates endpoint reachability. |
| 2 | Service Credentials | Input: one-time registration code. Calls `AuthProvider.ConsumeRegistrationCode()`. |
| 3 | Player ID Hasher | Input: pepper (min 5 chars). Hash algorithm displayed as "argon2" (read-only). |
| 4 | Use Players DB | Radio buttons: Yes / No. |
| 5 | Done | Success message. Transitions to RunningApp. |
| 6 | Error | Error display with retry/back options. |

---

## 3. Navigation

- **Forward:** `POST /setup/next` — validates current step, advances.
- **Backward:** `POST /setup/back` — returns to previous step, values preserved.
- Steps proceed sequentially. No skipping.
- Back button hidden on Welcome step.

### Completion Flow (Step 4 → Done)

1. Assemble complete config from all wizard values.
2. Validate the full config.
3. Write config to TOML file via config writer closure.
4. Call `Application.SetState()` with RunningApp builder.
5. Return StepDone.
6. If any step fails → return StepError with details.

---

## 4. Route Handlers

**`GET /setup`:** Render current step's template via SetupApp.GetCurrentState().

**`POST /setup/next`:**
1. Read form data.
2. Call appropriate SetupApp method (e.g., `SetServiceEndpoint(endpoint, env)`).
3. On success → render next step HTML.
4. On failure → re-render current step with error.

**`POST /setup/back`:**
1. Call `SetupApp.GoBackFrom(currentStep)`.
2. Render previous step with preserved values.

### htmx Integration

Setup pages use htmx for step transitions:
```html
<form hx-post="/setup/next" hx-target="#wizard-content" hx-swap="innerHTML">
    <!-- step content -->
</form>
```

---

## 5. Templates

Each step is a separate templ component. All in `internal/server/pages/`.

Templates receive step-specific data (current values, error messages, reason text) and render the appropriate form.

### Step Template Contract

Each template receives a struct implementing `SetupStepInfo` plus step-specific fields:
- Welcome: reason message (string, may be empty)
- Endpoint: endpoint value, environment value, error message
- Service Credentials: error message
- Player ID Hasher: pepper value, error message
- Use Players DB: current selection
- Done: success message
- Error: error message, back/retry buttons

---

## 6. State Redirection

All non-setup routes redirect to `/setup` when app is in SetupApp state:
- `GET /` → redirect `/setup`
- `GET /login` → redirect `/setup`
- etc.

Only exceptions: `/setup`, `/setup/{action}`, `/health`, static assets.

---

## Tests

| Test | Description |
|------|-------------|
| Full wizard flow | Navigate all 5 steps forward → Done, verify config written |
| Backward navigation preserves values | Set endpoint → go back → go forward → endpoint still set |
| Step validation failure | Invalid pepper (< 5 chars) → re-render with error |
| Skip prevention | Cannot jump from step 1 to step 4 |
| Recovery reason displayed | Create SetupApp with reason → Welcome step shows it |
| State transition on Done | After Done → Application in RunningApp state |
| Redirect to /setup | Request to `/` when in SetupApp → redirect |

## Acceptance Criteria

- [ ] All 7 step states render correctly
- [ ] Forward navigation validates before advancing
- [ ] Backward navigation preserves values
- [ ] Step skipping prevented
- [ ] Done step writes config and transitions to RunningApp
- [ ] Error step shows message with retry/back
- [ ] Non-setup routes redirect to /setup during SetupApp
- [ ] Dispatch uses map pattern, no deep nested switches
- [ ] All tests pass
