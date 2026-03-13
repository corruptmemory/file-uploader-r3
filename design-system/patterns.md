# Component Patterns

This document describes the CSS class patterns and templ structures used in the application. All styles reference design tokens from `tokens.css` -- never use raw color or spacing values.

CSS is split into two files loaded in this order:
1. `tokens.css` -- custom properties only (`:root` block)
2. `app.css` -- reset, utilities, and component classes

---

## Utility Classes

The design system provides a small set of utility classes. Use these for one-off adjustments; prefer component classes for repeated patterns.

### Flex

| Class | Effect |
|---|---|
| `.flex` | `display: flex` |
| `.flex-col` | `flex-direction: column` |
| `.flex-row` | `flex-direction: row` |
| `.items-center` | `align-items: center` |
| `.justify-between` | `justify-content: space-between` |
| `.gap-sm` | `gap: var(--space-sm)` |
| `.gap-md` | `gap: var(--space-md)` |

### Spacing

Margin and padding utilities follow the pattern `.{m|p}{t|b|l|r}-{xs|sm|md|lg|xl|xxl}`.

- `.m-md` = `margin: var(--space-md)`
- `.mt-lg` = `margin-top: var(--space-lg)`
- `.p-xl` = `padding: var(--space-xl)`
- `.mb-sm` = `margin-bottom: var(--space-sm)`

All six sizes are available: `xs` (0.25rem), `sm` (0.5rem), `md` (1rem), `lg` (1.5rem), `xl` (2rem), `xxl` (3rem).

### Text

| Class | Effect |
|---|---|
| `.text-center` | `text-align: center` |
| `.text-left` | `text-align: left` |
| `.text-right` | `text-align: right` |
| `.text-muted` | `color: var(--color-text-muted)` |
| `.text-sm` | `font-size: var(--font-size-sm)` |
| `.text-lg` | `font-size: var(--font-size-lg)` |
| `.font-bold` | `font-weight: 700` |

### Display and Width

| Class | Effect |
|---|---|
| `.block` | `display: block` |
| `.inline-block` | `display: inline-block` |
| `.hidden` | `display: none` |
| `.w-full` | `width: 100%` |
| `.w-auto` | `width: auto` |

---

## Navigation Bar

The navbar is a solid `--color-primary` (#1779ba) horizontal bar across the top of every authenticated page. It contains a brand link on the left and nav links on the right.

### CSS classes

- `.navbar` -- the outer `<nav>`, sets background color, padding, and shadow
- `.navbar-brand` -- the app name link, bold, `--font-size-lg`, white
- `.navbar-nav` -- `<ul>` containing nav items, displayed as a flex row with `--space-md` gap
- `.navbar-nav a` -- nav links, `--color-nav-link` (white at 85% opacity), small font, with `--border-radius` pill padding
- `.navbar-nav a:hover` -- full white text, `--color-nav-link-hover-bg` background
- `.navbar-nav a.active` -- full white text, `--color-nav-link-active-bg` background (slightly more opaque)
- `.btn-logout` -- transparent background, subtle white border, used for the logout action in the nav

### Templ structure

```
<nav class="navbar">
  <div class="container">
    <a href="/" class="navbar-brand">File Uploader</a>
    <ul class="navbar-nav">
      <li><a href="/" class={ if active { "active" } }>Dashboard</a></li>
      <li><a href="/archived">Archive</a></li>
      <li><a href="/settings">Settings</a></li>
      <li><a href="/players-db">Players DB</a></li>
      <li><a href="/logout" class="btn-logout">Logout</a></li>
    </ul>
  </div>
</nav>
```

The active page is tracked via a string (`"dashboard"`, `"archived"`, `"settings"`, `"players-db"`) and the matching link gets the `.active` class.

---

## Buttons

All buttons use the `.btn` base class plus a variant class.

### CSS classes

- `.btn` -- base: inline-block, `--space-sm`/`--space-md` padding, `--border-radius`, transparent border, 0.15s background transition
- `.btn-primary` -- `--color-primary` background, white text. Hover: `--color-primary-dark`
- `.btn-secondary` -- `--color-bg-alt` background, `--color-text` text, `--color-border` border. Hover: `--color-border` background
- `.btn-danger` -- `--color-danger` background, white text. Hover: `--color-danger-dark`
- `.btn-sm` -- smaller padding (`--space-xs`/`--space-sm`) and `--font-size-sm`
- `.btn-logout` -- transparent background, `--color-nav-link` text, `--color-nav-border-subtle` border (used only in navbar)

### Usage

```html
<button class="btn btn-primary">Save</button>
<button class="btn btn-secondary">Cancel</button>
<button class="btn btn-danger">Delete</button>
<button class="btn btn-primary btn-sm">Small Action</button>
<a href="/dashboard" class="btn btn-primary">Go to Dashboard</a>
```

Buttons can be `<button>` or `<a>` elements. Full-width buttons (e.g., login form) use inline `width: 100%` or the `.w-full` utility.

---

## Cards

Cards are bordered content containers with a subtle shadow.

### CSS classes

- `.card` -- `--border-width` solid `--color-border`, `--border-radius-lg`, `--space-lg` padding, `--color-bg` background, `--shadow-sm`, `--space-lg` bottom margin

### Usage

```html
<div class="card">
  <div class="flex justify-between items-center mb-sm">
    <span class="font-bold">filename.csv</span>
    <span class="text-muted text-sm">Player Registrations</span>
  </div>
  <!-- card content -->
</div>
```

Cards are used for the processing display on the dashboard, the players DB status panel, and other self-contained content blocks. They do not have header/body sub-divisions -- content goes directly inside.

---

## Sections

Sections are logical groupings within a page, each with a titled header.

### CSS classes

- `.section` -- `--space-xl` bottom margin
- `.section-title` -- `--font-size-lg`, bold, `--space-md` bottom margin, 2px solid `--color-border` bottom border

### Usage

```html
<div class="section">
  <h2 class="section-title">Upload Files</h2>
  <!-- section content -->
</div>
```

Every major content area on the dashboard, archive, settings, and players DB pages is wrapped in a `.section` with a `.section-title`.

---

## Tables

Data tables display file lists, queue contents, and archive results.

### CSS classes

- `.table-wrapper` -- `overflow-x: auto` wrapper for horizontal scrolling on small screens
- `.data-table` -- full width, collapsed borders, `--font-size-sm`
- `.data-table thead` -- sticky top, z-index 1
- `.data-table th` -- `--color-bg-alt` background, `--space-sm`/`--space-md` padding, bold, nowrap
- `.data-table td` -- `--space-sm`/`--space-md` padding, bordered
- `.data-table tbody tr:nth-child(even)` -- `--color-bg-alt` striped rows
- `.data-table tbody tr:hover` -- `--color-border` hover background
- `.row-success` -- `--color-row-success` (#d4edda) green background for successful rows
- `.row-failure` -- `--color-row-failure` (#f8d7da) red background, pointer cursor (clickable for failure details)
- `.row-failure:hover` -- `--color-row-failure-hover` slightly darker red

### Templ structure

```html
<div class="table-wrapper">
  <table class="data-table">
    <thead>
      <tr>
        <th>Filename</th>
        <th>Type</th>
        <th>Status</th>
        <th>Timestamp</th>
      </tr>
    </thead>
    <tbody>
      <tr class="row-success">
        <td>file.csv</td>
        <td>Player Registrations</td>
        <td><span class="badge badge-success">Success</span></td>
        <td>2025-01-15 14:30:00</td>
      </tr>
      <tr class="row-failure" data-failure-id="abc123">
        <td>bad.csv</td>
        <td>Unknown</td>
        <td><span class="badge badge-failure">Failed</span></td>
        <td>2025-01-15 14:31:00</td>
      </tr>
    </tbody>
  </table>
</div>
```

Always wrap tables in `.table-wrapper` to handle overflow on mobile.

---

## Badges / Status Indicators

Badges are inline labels showing success or failure status.

### CSS classes

- `.badge` -- inline-block, `--space-xs`/`--space-sm` padding, `--border-radius`, `--font-size-sm`, bold, white text, line-height 1
- `.badge-success` -- `--color-success` (#28a745) green background
- `.badge-failure` -- `--color-danger` (#dc3545) red background

### Usage

```html
<span class="badge badge-success">Success</span>
<span class="badge badge-failure">Failed</span>
```

Badges appear inside table cells to indicate file processing outcomes.

---

## Forms

Forms use `.form-group` wrappers for each field.

### CSS classes

- `.form-group` -- `--space-md` bottom margin
- `.form-group label` -- block display, `--space-xs` bottom margin, bold, `--font-size-sm`
- `.form-group input[type="text"]`, `input[type="password"]`, `input[type="email"]`, `select`, `textarea` -- full width, `--space-sm` padding, `--border-width` solid `--color-border`, `--border-radius`, `--font-size-base`
- Focus state: `--color-primary` border, 2px `--color-primary-ring` box-shadow, no outline
- `.form-group small` -- block, `--space-xs` top margin, `--color-text-muted`, `--font-size-sm` (used for help text)

### Templ structure

```html
<div class="form-group">
  <label for="pepper">Pepper</label>
  <input type="text" id="pepper" name="pepper" value="..." required minlength="5"/>
  <small>Minimum 5 characters. Used for hashing player IDs.</small>
</div>
```

Form submit buttons are placed after the last `.form-group`, not inside one.

### Settings page sections

The settings page uses `.settings-section` divs to group related form fields under `<h3>` headings. Each section is a logical group (API Endpoint, Service Credentials, Player ID Hasher, Use Players DB) within a single `.section`.

---

## Upload / Drop Zone

The drag-and-drop file upload area.

### CSS classes

- `.drop-zone` -- 2px dashed `--color-border`, `--border-radius-lg`, `--space-xl` padding, centered text, `--color-bg` background, pointer cursor, `--space-lg` bottom margin
- `.drop-zone:hover` -- border becomes `--color-primary`
- `.drop-zone.drag-over` -- border becomes `--color-primary`, background becomes `--color-primary-tint` (very subtle blue)
- `.drop-zone-label` -- `--color-text-muted`, `--font-size-lg`, `--space-md` bottom margin
- `.drop-zone-file-input` -- `display: none` (hidden native file input)
- `.drop-zone-files` -- container for selected file list, `--space-md` top margin
- `.drop-zone-actions` -- flex row, centered, `--space-sm` gap, `--space-md` top margin (holds Select/Upload/Clear buttons)
- `.drop-zone-message` -- feedback area, `--space-sm`/`--space-md` padding, `--border-radius`, `--font-size-sm`
- `.drop-zone-message.success` -- `--color-row-success` background, `--color-success-text` text
- `.drop-zone-message.error` -- `--color-row-failure` background, `--color-danger-text` text

### Templ structure

```html
<div id="drop-zone" class="drop-zone">
  <div class="drop-zone-label">Drag and drop CSV files here, or click to select</div>
  <input type="file" class="drop-zone-file-input" accept=".csv" multiple/>
  <div class="drop-zone-actions">
    <button type="button" class="btn btn-primary btn-sm drop-zone-pick-btn">Select Files</button>
    <button type="button" class="btn btn-primary btn-sm drop-zone-upload-btn">Upload</button>
    <button type="button" class="btn btn-secondary btn-sm drop-zone-clear-btn">Clear</button>
  </div>
  <div class="drop-zone-files"></div>
  <div class="drop-zone-message" style="display:none"></div>
</div>
```

JavaScript handles the drag events (adding/removing `.drag-over`), populating `.drop-zone-files` with the selected filenames, and showing upload results in `.drop-zone-message`.

---

## Progress Bars

Progress bars show file processing and upload completion.

### CSS classes

- `.progress-bar-container` -- full width, 20px height, `--color-bg-alt` background, `--border-radius`, overflow hidden, `--border-width` solid `--color-border`
- `.progress-bar-fill` -- `--color-primary` background, 0.3s width transition, `--border-radius`, flex centered, white bold `--font-size-sm` text, `min-width: 2em`

### Usage

```html
<div class="progress-bar-container">
  <div class="progress-bar-fill" style="width: 45%">45%</div>
</div>
```

The width is set via inline style (from the server). The percentage text is rendered inside the fill element. Progress bars appear in the processing card and in the uploading table cells.

---

## Modals

Modals are full-screen overlays used for session expiry warnings and failure details.

### CSS classes

- `.modal-overlay` -- fixed position, full viewport, `--color-overlay` (50% black) background, z-index 1000, flex centered, `display: none` by default
- `.modal-overlay.active` -- `display: flex` (shown)
- `.modal-content` -- `--color-bg` background, `--border-radius-lg`, `--space-xl` padding, `--shadow-md`, max-width 500px, 90% width, centered text
- `.modal-content h2` -- `--space-md` bottom margin
- `.modal-content p` -- `--space-lg` bottom margin, `--color-text-muted`
- `.modal-content .btn` -- `--space-xs` horizontal margin

### Templ structure (session modal)

```html
<div id="session-modal" class="modal-overlay">
  <div class="modal-content">
    <h2>Session Expiring</h2>
    <p>Your session is about to expire.</p>
    <button class="btn btn-primary modal-close-btn">Stay Logged In</button>
  </div>
</div>
```

### Templ structure (failure details modal)

```html
<div id="failure-modal" class="modal-overlay">
  <div class="modal-content">
    <!-- content loaded via htmx -->
  </div>
</div>
```

Modals are shown by adding `.active` to the overlay. The failure modal loads its content via `hx-get` into `.modal-content`. Close buttons use `.modal-close-btn` and JavaScript to remove the `.active` class.

### Failure details content

```html
<div class="failure-details">
  <h3>Failure Details</h3>
  <div class="detail-row">
    <span class="detail-label">Filename:</span>
    <span class="detail-value">file.csv</span>
  </div>
  <div class="detail-row">
    <span class="detail-label">Phase:</span>
    <span class="detail-value">validation</span>
  </div>
  <div class="failure-reason">Error message here...</div>
  <button class="btn btn-secondary modal-close-btn mt-md">Close</button>
</div>
```

- `.failure-details` -- left-aligned text
- `.detail-row` -- flex row, `--space-sm` bottom margin
- `.detail-label` -- bold, 120px min-width, `--color-text-muted`
- `.detail-value` -- flex: 1
- `.failure-reason` -- `--space-md` padding, `--color-row-failure` background, `--border-radius`, monospace font, `--font-size-sm`, pre-wrap, break-all

---

## Alerts / Messages

Alerts provide feedback after form submissions.

### CSS classes

- `.login-error` -- `--color-row-failure` background, `--color-danger-text`, `--space-sm`/`--space-md` padding, `--border-radius`, `--space-md` bottom margin, `--font-size-sm`
- `.setup-error` -- same styling as `.login-error` (failure background with danger text)
- `.setup-reason` -- `--color-row-success` background, `--color-success-text`, same padding/radius (used for informational messages in the setup wizard)
- `.alert.alert-success` -- used on settings page for save confirmation
- `.alert.alert-danger` -- used on settings page for registration code errors

### Usage

```html
<!-- Error message -->
<div class="login-error">Invalid username or password</div>

<!-- Success message in setup -->
<div class="setup-reason"><p>Explanation of why setup is needed.</p></div>

<!-- Error message in setup -->
<div class="setup-error"><p>Something went wrong.</p></div>
```

---

## Empty States

Shown when a table or list has no data.

### CSS classes

- `.empty-state` -- centered text, `--space-lg` padding, `--color-text-muted`, italic

### Usage

```html
<div class="empty-state">No files in queue</div>
```

Used in place of tables when there are no queued files, no files processing, no uploading files, or no recently finished files.

---

## Animations

### Fade out

- `.fade-out` -- 5-second opacity fade from 1 to 0, using `@keyframes fadeOut`

Used for flash messages that should auto-dismiss.

---

## Login Page

The login page has its own standalone layout (no navbar) with a centered card.

### CSS classes

- `.login-page` -- flex, centered both axes, `min-height: 100vh`, `--color-bg-alt` background
- `.login-card` -- max-width 400px, full width, `--space-xl` padding, `--color-bg` background, `--border-radius-lg`, `--shadow-md`
- `.login-card h1` -- centered, `--space-lg` bottom margin
- `.login-card .btn` -- full width
- `.login-error` -- error alert (see Alerts above)

### Templ structure

```html
<div class="login-page">
  <div class="login-card">
    <h1>File Uploader</h1>
    <div class="login-error">Error message if any</div>
    <form hx-post="/login" hx-target="#login-form" hx-swap="outerHTML">
      <div class="form-group">
        <label for="username">Username</label>
        <input type="text" id="username" name="username" required/>
      </div>
      <div class="form-group">
        <label for="password">Password</label>
        <input type="password" id="password" name="password" required/>
      </div>
      <button type="submit" class="btn btn-primary">Login</button>
    </form>
  </div>
</div>
```

The login form uses htmx to swap itself on validation errors (re-renders the form with the error message without a full page reload).

---

## Setup Wizard

The setup wizard is a standalone page (no navbar) with a centered, narrow-width stepped form.

### CSS classes

- `.setup-wizard` -- max-width 600px, `--space-xxl` auto margin, `--space-md` horizontal padding
- `.setup-wizard h1` -- centered, `--space-xl` bottom margin
- `.setup-step` -- `--color-bg` background, `--border-width` solid `--color-border`, `--border-radius-lg`, `--space-xl` padding, `--shadow-sm`
- `.setup-actions` -- flex row, `--space-sm` gap, right-justified (`flex-end`), `--space-lg` top margin

### Templ structure

```html
<div class="setup-wizard">
  <h1>Setup Wizard</h1>
  <div id="wizard-content">
    <div class="setup-step">
      <h2>Service Endpoint</h2>
      <div class="setup-error"><p>Error if any</p></div>
      <form hx-post="/setup/next" hx-target="#wizard-content" hx-swap="innerHTML">
        <input type="hidden" name="current_step" value="1"/>
        <div class="form-group">
          <label for="endpoint">Endpoint URL</label>
          <input type="text" id="endpoint" name="endpoint" required/>
        </div>
        <div class="setup-actions">
          <button type="button" class="btn btn-secondary" hx-post="/setup/back">Back</button>
          <button type="submit" class="btn btn-primary">Next</button>
        </div>
      </form>
    </div>
  </div>
</div>
```

Navigation between steps uses htmx to swap `#wizard-content` innerHTML. Back buttons use `hx-post="/setup/back"` with a hidden `current_step` value.
