# 13 — Dashboard, SSE, and Client-Side JavaScript

**Dependencies:** 10-application-state-machine.md (RunningApp, CSVProcessingState, EventSubscription, DataUpdateEvent), 11-auth-and-server.md (route table, WebApp, session middleware, cookie management).

**Design System:** Follow the design system in `design-system/` for all visual implementation. Use `tokens.css` for colors, spacing, and fonts. Follow `patterns.md` for component patterns. Follow `layout.md` for page structure and responsive behavior.

**Produces:** Dashboard page template, SSE handler, `sse.js` htmx extension, `app.js` client logic, `tokens.css`, `app.css`.

---

## 1. Dashboard Page (`GET /`)

The main operational page. Requires authentication and RunningApp state.

### Layout

Four live-updating sections plus a file upload zone:

1. **File Upload Zone** — drag-and-drop + file picker
2. **Queued Files** — table of files awaiting processing
3. **Processing File** — current file with progress bar
4. **Uploading Files** — table of files being uploaded with progress bars
5. **Recently Finished** — last 5 completed files with success/failure badges

### File Upload Zone

- Dashed-border drop area. Highlights on dragover (border color change + subtle background tint).
- File picker button as alternative to drag-and-drop.
- Client-side validation:
  - Only `.csv` extension accepted (case-insensitive).
  - Maximum 50 MB per file.
  - Invalid files → inline error message.
- After selection: display filenames and count.
- **Upload** button POSTs files to `/upload` as `multipart/form-data`.
- **Clear** button removes selected files and resets the zone.
- Success/failure messages fade out after 5 seconds (CSS animation).

### Upload Handler (`POST /upload`)

1. Validate session (withStateAndSession middleware).
2. Use `http.MaxBytesReader` to enforce 50 MB server-side limit.
3. Parse multipart form.
4. For each file: save to upload directory, call `RunningApp.ProcessUploadedCSVFile()`.
5. Return success/error HTML fragment (htmx swap).

---

## 2. SSE Server Handler (`GET /events`)

### HTTP Setup

1. Set headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`.
2. Assert `http.Flusher` from ResponseWriter. If unavailable → return HTTP 500.
3. Flush headers immediately.

### Event Loop

1. Call `RunningApp.Subscribe()` → `EventSubscription` with ID and `<-chan DataUpdateEvent`.
2. Enter `select` loop:
   - **DataUpdateEvent received:** Render 4 templ components from `CSVProcessingState`. Flatten each to single-line HTML (strip `\n`). Write as named SSE events. Flush.
   - **Request context cancelled (client disconnect):** Call `RunningApp.Unsubscribe(id)`. Return.

### SSE Wire Format

```
event: queued
data: <single-line HTML>

event: processing
data: <single-line HTML>

event: uploading
data: <single-line HTML>

event: recently-finished
data: <single-line HTML>

```

Each block: `event: <name>\n`, `data: <html>\n`, blank line `\n`. HTML must be on one `data:` line — no embedded newlines.

### SSE Templ Components

Four components, each receives `CSVProcessingState`:

| Component | Source Data | Renders |
|-----------|-----------|---------|
| QueuedTable | `QueuedFiles []FileMetadata` | Table rows: filename, uploaded by, timestamp |
| ProcessingDisplay | `ProcessingFile *CSVProcessingFile` | Filename, CSV type, progress bar with percent. Nil → "No file currently processing" |
| UploadingTable | `UploadingFiles []CSVUploadingFile` | Table rows: filename, type, progress bar |
| RecentlyFinishedTable | `FinishedFiles []CSVFinishedFile` (last 5) | Table rows: filename, type, status badge, timestamp. Failure rows are clickable |

Failure rows in RecentlyFinished trigger `GET /failure-details/{record-id}` → modal content.

---

## 3. SSE Client (`static/js/sse.js`)

Custom htmx extension — NOT the official npm SSE extension. Plain browser JS, no build step.

### Registration

```js
htmx.defineExtension("sse", { ... })
```

### HTML Attributes

- `sse-connect="<url>"` — establish EventSource connection
- `sse-swap="<event-name>"` — mark element as swap target for named event

### Behavior

1. On `htmx:load`, scan for elements with `sse-connect`.
2. Create `EventSource` with `withCredentials: true`.
3. Find all descendant elements with `sse-swap`. Add event listener per event name.
4. On named event: find matching `sse-swap` element, set `innerHTML` to event data, call `htmx.process()` on element.
5. On EventSource error: if page is unloading → ignore. Otherwise close, emit `htmx:sseError`, schedule reconnect.
6. On EventSource open: reset backoff to 1 second.
7. On `htmx:beforeCleanupElement`: close EventSource.

### Reconnection

Exponential backoff: 1s → 2s → 4s → 8s → ... → 128s (cap). Reset to 1s on successful connection.

### Page Unload

Listen for `beforeunload`. Suppress error events during unload.

---

## 4. Application JavaScript (`static/js/app.js`)

Plain browser JS, no modules, no build step. Served from `/js/app.js`.

### Session Management

- `getCookie(name)` — return cookie value or null.
- `checkSession()` — parse `session-expires` cookie (RFC 1123), return seconds until expiry (-1 if no session).
- `extendSession()` — `fetch("GET /api/extend")` with credentials.
- `showSessionModal()` / `hideSessionModal()` — toggle `.active` on modal overlay.
- `startSessionTimer()` — 1-second interval. At 30 seconds remaining → show modal. At 0 → navigate to `/logout`.
- `setupAutoExtension()` — listen for `mousedown`, `keydown`, `input`, `scroll`. On activity, if 60-second cooldown elapsed and session exists → call `extendSession()`.

### File Upload

- `prepFileDrop()` — initialize upload zone:
  - `dragover` → add `drag-over` class, `preventDefault`.
  - `dragleave` → remove `drag-over` class.
  - `drop` → validate files (`.csv`, ≤50MB), update display.
  - File picker button → trigger hidden `<input type="file" accept=".csv" multiple>`.
  - Upload button → `FormData` + `fetch("POST /upload")` with credentials.
  - Clear button → reset.
- `isCSVFile(filename)` — true if ends with `.csv` (case-insensitive).

### htmx Integration

Listen for `htmx:afterSwap`. Attach click handlers for modal trigger elements in swapped content.

### Initialization

On `DOMContentLoaded`:
1. If session cookie exists → `startSessionTimer()`.
2. `setupAutoExtension()`.
3. If `#drop-zone` exists → `prepFileDrop()`.

---

## 5. Dashboard HTML Structure

```html
<div hx-ext="sse" sse-connect="/events">
  <!-- Upload Zone -->
  <div id="drop-zone">...</div>

  <!-- Queued -->
  <div id="queued-table-body" sse-swap="queued"></div>

  <!-- Processing -->
  <div id="processing-body" sse-swap="processing"></div>

  <!-- Uploading -->
  <div id="uploading-table-body" sse-swap="uploading"></div>

  <!-- Recently Finished -->
  <div id="finished-table-body" sse-swap="recently-finished"></div>
</div>
```

SSE connection established on page load. Server sends initial state snapshot as first event set.

---

## 6. CSS

### Design Tokens (`static/css/tokens.css`)

```css
:root {
    --color-primary: #1779ba;
    --color-primary-dark: #135e96;
    --color-success: #28a745;
    --color-danger: #dc3545;
    --color-warning: #ffc107;
    --color-bg: #ffffff;
    --color-bg-alt: #f8f9fa;
    --color-text: #212529;
    --color-text-muted: #6c757d;
    --color-border: #dee2e6;
    --color-row-success: #d4edda;
    --color-row-failure: #f8d7da;

    --space-xs: 0.25rem;
    --space-sm: 0.5rem;
    --space-md: 1rem;
    --space-lg: 1.5rem;
    --space-xl: 2rem;
    --space-xxl: 3rem;

    --font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    --font-size-sm: 0.875rem;
    --font-size-base: 1rem;
    --font-size-lg: 1.25rem;
    --font-size-xl: 1.5rem;

    --border-radius: 4px;
    --border-radius-lg: 8px;
    --border-width: 1px;

    --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.05);
    --shadow-md: 0 4px 6px rgba(0, 0, 0, 0.1);

    --max-width: 1200px;
    --sidebar-width: 250px;
}
```

### Utility Classes

Provide these families in `app.css`:

- **Flex:** `.flex`, `.flex-col`, `.flex-row`, `.items-center`, `.justify-between`, `.gap-sm`, `.gap-md`
- **Spacing** (for xs/sm/md/lg/xl/xxl): `.m-{size}`, `.p-{size}`, `.mt-{size}`, `.mb-{size}`, `.ml-{size}`, `.mr-{size}`
- **Text:** `.text-center`, `.text-left`, `.text-right`, `.text-muted`, `.text-sm`, `.text-lg`, `.font-bold`
- **Display:** `.block`, `.inline-block`, `.hidden`
- **Width:** `.w-full`, `.w-auto`

### Key Component Styles

All reference design tokens via `var(--token)`, never hardcoded values.

**Drop zone:** Dashed border, highlights on `.drag-over` with primary color border + 5% opacity background.

**Progress bar:** 20px height, `--color-bg-alt` track, `--color-primary` fill, 0.3s width transition.

**Modal overlay:** Fixed position, full viewport, semi-transparent black background, centered content with `--shadow-md`.

**Tables:** Full width, collapsed borders, sticky header, alternating row backgrounds, hover highlight.

**Status badges:** Inline-block, rounded, white text on `--color-success` (green) or `--color-danger` (red).

**Fade-out animation:** `@keyframes fadeOut` from opacity 1→0 over 5 seconds. Class `.fade-out`.

### Responsive Breakpoints

- **Desktop** (> 1024px): Multi-column layout, `--max-width: 1200px`.
- **Tablet** (768px–1024px): `--max-width: 100%`, adjust to fewer columns.
- **Mobile** (< 768px): Single column, increased touch targets, horizontally scrollable tables.

**Constraint:** Zero CSS frameworks. All colors, spacing, typography from design tokens.

---

## Tests

| Test | Description |
|------|-------------|
| SSE handler sets correct headers | Verify Content-Type, Cache-Control, Connection headers |
| SSE sends initial state on subscribe | Connect → receive 4 named events immediately |
| SSE disconnect triggers unsubscribe | Close connection → subscriber removed |
| SSE returns 500 without Flusher | Mock ResponseWriter without Flusher → 500 |
| Upload enforces 50MB limit | Oversized request → 413 response |
| Upload requires authentication | No session cookie → redirect to /login |
| Upload accepts valid CSV | POST multipart with .csv file → 200, file queued |
| Failure details modal | GET /failure-details/{id} → returns HTML fragment |

## Acceptance Criteria

- [ ] Dashboard renders upload zone + 4 SSE sections
- [ ] File upload validates .csv extension and 50MB limit client-side and server-side
- [ ] SSE connection populates all sections on initial connect
- [ ] SSE updates swap into correct DOM elements by event name
- [ ] SSE HTML is single-line (no embedded newlines)
- [ ] SSE reconnects with exponential backoff (1s to 128s cap)
- [ ] Session auto-extension fires on activity with 60-second cooldown
- [ ] Session warning modal appears at 30 seconds before expiry
- [ ] Clicking failure row opens details modal
- [ ] All CSS uses design tokens, zero frameworks
- [ ] Responsive at 768px and 1024px breakpoints
- [ ] All tests pass
