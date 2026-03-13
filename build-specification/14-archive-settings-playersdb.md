# 14 — Archive, Settings, Players DB, and Login Pages

**Dependencies:** 10-application-state-machine.md (RunningApp interface, CSVFinishedFile, FinishedStatus, CSVType), 11-auth-and-server.md (route table, AuthProvider, session middleware), 08-player-db.md (PlayerDB, ConcurrentPlayerDB, snapshot), 03-configuration.md (ApplicationConfig, ApplicationConfigErrors, ValidateSettableValues).

**Design System:** Follow the design system in `design-system/` for all visual implementation. Use `tokens.css` for colors, spacing, and fonts. Follow `patterns.md` for component patterns. Follow `layout.md` for page structure and responsive behavior.

**Produces:** Page templates and route handlers for Archive, Settings, Players DB, Login, and Logout.

---

## 1. Archive Page (`GET /archived`)

Search and filter interface for completed file uploads.

### Search Controls

Three filters in a single form:

1. **Status dropdown** — Options: All, Success, Failure. Maps to `FinishedStatus` enum.
2. **Type dropdown** — Options: All, plus each of the 10 CSV types.
3. **Free-text search** — Case-insensitive substring match against filename and uploader name.

### htmx Integration

```html
<form hx-post="/search-archived"
      hx-trigger="change, keyup changed delay:300ms"
      hx-target="#archive-results"
      hx-swap="innerHTML">
    <!-- filter controls -->
</form>
<div id="archive-results">
    <!-- results table body swapped here -->
</div>
```

- `change` trigger fires on dropdown selection (immediate).
- `keyup changed delay:300ms` fires on text input with 300ms debounce.

### Results Table

| Column | Description |
|--------|-------------|
| File | Original filename |
| CSV Type | Detected CSV type |
| Status | Success or Failure badge |
| Uploaded By | Username |
| Processed At | Processing completion timestamp |
| Uploaded At | File upload timestamp |
| Failure Phase | Which pipeline phase failed (empty for success) |

### Behavior

- Success rows: green background (`var(--color-row-success)` / `#d4edda`).
- Failure rows: red background (`var(--color-row-failure)` / `#f8d7da`).
- Clicking a failure row → `GET /failure-details/{record-id}` → swap into modal container.
- Initial page load: show all archived files, no filters, ordered by processed timestamp descending.
- `POST /search-archived` reads form values, calls `RunningApp.SearchFinished(status, csvTypes, search)`, returns rendered table rows (not full page).

### Failure Details Modal (`GET /failure-details/{record-id}`)

1. Call `RunningApp.GetFinishedDetails(recordID)`.
2. Return HTML fragment showing: filename, CSV type, failure phase, failure reason, timestamps.
3. Modal displayed via `.modal-overlay.active` toggle.

### Constraints

- Free-text search is simple case-insensitive substring, not full-text.
- No pagination — all matching results returned.

---

## 2. Settings Page (`GET /settings`, `POST /settings`)

Display and edit configurable values that can be changed after setup.

### Four Sections

1. **API Endpoint** — Read-only display of environment name and endpoint URL. Set during setup or via config file.

2. **Service Credentials** — Registration code input. Submitting calls `AuthProvider.ConsumeRegistrationCode()`. Shows success or error. This is a separate htmx form from the main settings form.

3. **Player ID Hasher** — Text input for pepper (min 5 chars). Hash algorithm displayed as read-only "argon2".

4. **Use Players DB** — Radio buttons: Yes / No.

### Behavior

- `GET /settings`: Render page with current values from `RunningApp.GetConfig()`.
- `POST /settings`: Validate via `ApplicationConfig.ValidateSettableValues()`.
  - Validation fails → re-render with inline error messages from `ApplicationConfigErrors` struct (per-field errors).
  - Validation succeeds → save via config writer closure → re-render with success message.
- Registration code submission is a dedicated htmx POST (separate from main form) to avoid conflicting.

### Constraints

- API Endpoint is display-only on this page.
- Pepper minimum 5 chars — validated both client-side (`minlength`) and server-side.
- Hash algorithm always "argon2", not editable.

---

## 3. Players DB Page (`GET /players-db`)

Status display and download for the player deduplication database.

### Behavior

- **Enabled:** Show entry count, last updated timestamp, and download button.
- **Disabled:** Show message that feature is off + link to Settings page to enable.
- `GET /download-players-db`: Call `RunningApp.DownloadPlayersDB(orgPlayerHash, orgPlayerIDPepper, response)`.
  - Set `Content-Disposition: attachment; filename="players.db"`.
  - Stream content as response body.
  - This is a direct file download (standard `<a>` tag), not an htmx request.

---

## 4. Login Page (`GET /login`, `POST /login`)

### Form Fields

- Username text input.
- Password text input (`type="password"`).
- Conditional MFA field: rendered only when `RunningApp.MFARequired()` returns true.
- Submit button.

### Types

```go
type LoginErrorResponse struct {
    Username string  // preserve on error
    Password string  // always empty
    NeedsMFA bool    // whether to show MFA field
    Error    string  // error message
}
```

### Behavior

- `GET /login`: Render form. Check `MFARequired()` for conditional MFA field.
- `POST /login` via htmx (`hx-post="/login"`, form targets itself):
  - Success → set session cookies, return `HX-Redirect: /` header. htmx follows redirect.
  - Failure → return login form HTML with error message, username pre-filled, password always empty.

### Constraints

- Password never echoed back.
- MFA field visibility determined server-side, not client-side toggle.

---

## 5. Logout (`GET /logout`)

1. Clear both `session` and `session-expires` cookies (set expired).
2. Redirect to `/login`.

---

## Tests

| Test | Description |
|------|-------------|
| Archive shows all files initially | GET /archived → table with all finished files |
| Archive filters by status | POST /search-archived with status=failure → only failures |
| Archive filters by type | POST /search-archived with type=players → only Players files |
| Archive text search | POST /search-archived with search="test" → matching filenames |
| Archive debounce | Verify hx-trigger includes delay:300ms for keyup |
| Failure details modal | GET /failure-details/{id} → HTML fragment with details |
| Settings displays current values | GET /settings → form pre-populated from config |
| Settings validation error | POST /settings with short pepper → inline error |
| Settings save success | POST /settings with valid data → config written, success message |
| Registration code | POST registration code → ConsumeRegistrationCode called |
| Players DB enabled | GET /players-db → entry count + download button |
| Players DB disabled | GET /players-db → "feature off" message + settings link |
| Players DB download | GET /download-players-db → Content-Disposition header + file |
| Login renders MFA conditionally | MFARequired=true → MFA field visible |
| Login success | POST /login → HX-Redirect header set |
| Login failure | POST /login → error message, username preserved, password empty |
| Logout clears cookies | GET /logout → cookies expired, redirect to /login |

## Acceptance Criteria

- [ ] Archive page shows all finished files sorted by timestamp descending
- [ ] Archive filters work: status, type, and free-text search
- [ ] Free-text search debounces at 300ms, dropdowns trigger immediately
- [ ] Failure rows are clickable and open details modal
- [ ] Settings page displays all four sections with current values
- [ ] Settings validation shows inline per-field errors
- [ ] Registration code submission is a separate form action
- [ ] Players DB page shows enabled/disabled state correctly
- [ ] Download sets Content-Disposition header
- [ ] Login handles success (HX-Redirect), failure (error + preserved username), and MFA
- [ ] Logout clears both cookies and redirects
- [ ] All tests pass
