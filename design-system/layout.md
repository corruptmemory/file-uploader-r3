# Page Layout System

This document describes how pages are structured, including the overall layout shell, responsive behavior, and how different page types diverge in structure.

---

## CSS Loading Order

Every page loads two stylesheets in this order:

```html
<link rel="stylesheet" href="/css/tokens.css"/>
<link rel="stylesheet" href="/css/app.css"/>
```

`tokens.css` defines all custom properties. `app.css` provides the reset, utilities, and all component classes. No other CSS files are used.

---

## Global Reset

All elements use `box-sizing: border-box` with zeroed margins and padding. The `<html>` element sets:

- `font-family: var(--font-family)` -- system font stack
- `font-size: var(--font-size-base)` -- 1rem
- `color: var(--color-text)` -- #212529
- `background-color: var(--color-bg)` -- #ffffff
- `line-height: 1.5`

The `<body>` has `min-height: 100vh`.

Links are `--color-primary` with no underline; hover adds underline and darkens to `--color-primary-dark`.

---

## Page Types

The application has three distinct page layouts:

### 1. Authenticated pages (Layout template)

Used by: Dashboard, Archive, Settings, Players DB

```
+--------------------------------------------------+
| navbar (full width, --color-primary background)   |
|  .container: brand left, nav links right          |
+--------------------------------------------------+
| <main class="container page-content">             |
|   page-specific content                           |
| </main>                                           |
+--------------------------------------------------+
| modals (session expiry, failure details)           |
+--------------------------------------------------+
```

The Layout templ component wraps all authenticated pages:

```html
<body>
  <!-- Nav bar -->
  <nav class="navbar">
    <div class="container">...</div>
  </nav>
  <!-- Main content -->
  <main class="container page-content">
    { children... }
  </main>
  <!-- Modals (hidden by default) -->
  <div id="session-modal" class="modal-overlay">...</div>
  <div id="failure-modal" class="modal-overlay">...</div>
</body>
```

### 2. Login page (standalone)

No navbar, no modals. Full-viewport centered card.

```
+--------------------------------------------------+
| .login-page (flex centered, 100vh, bg-alt)        |
|                                                    |
|          +------------------------+                |
|          | .login-card            |                |
|          |  <h1>                  |                |
|          |  <form>               |                |
|          |    username            |                |
|          |    password            |                |
|          |    [Login]             |                |
|          +------------------------+                |
|                                                    |
+--------------------------------------------------+
```

### 3. Setup wizard (standalone)

No navbar, no modals. Centered narrow container with stepped form.

```
+--------------------------------------------------+
|                                                    |
|      .setup-wizard (max-width: 600px, centered)   |
|        <h1>Setup Wizard</h1>                       |
|        .setup-step (card-like bordered box)        |
|          <h2>Step Title</h2>                       |
|          <form>...</form>                          |
|          .setup-actions [Back] [Next]              |
|                                                    |
+--------------------------------------------------+
```

---

## Container

The `.container` class constrains content width and centers it:

```css
.container {
    max-width: var(--max-width);   /* 1200px */
    margin: 0 auto;
    padding: 0 var(--space-md);   /* 1rem horizontal padding */
}
```

This is used in two places:
1. Inside `.navbar` -- constrains nav content to 1200px
2. As `<main class="container page-content">` -- constrains page content to 1200px

The `.page-content` class adds `--space-xl` (2rem) vertical padding.

---

## Dashboard Grid

The dashboard uses a 2-column CSS grid for its four status sections (Queued, Processing, Uploading, Recently Finished):

```css
.dashboard-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-lg);          /* 1.5rem */
}
```

The upload zone sits above the grid as a full-width section. The grid contains four `.section` elements laid out 2x2.

```
+---------------------------------------------+
| .section: Upload Files (full width)          |
|   .drop-zone                                 |
+---------------------------------------------+
| .dashboard-grid                              |
| +--------------------+ +-------------------+ |
| | .section: Queued   | | .section: Process | |
| |   data-table       | |   card + progress | |
| +--------------------+ +-------------------+ |
| +--------------------+ +-------------------+ |
| | .section: Upload   | | .section: Recent  | |
| |   data-table       | |   data-table      | |
| +--------------------+ +-------------------+ |
+---------------------------------------------+
```

---

## Archive Page

Single-column layout. A filter form sits above the results table.

```
+---------------------------------------------+
| .section: Archived Files                     |
|   <form> .filter-controls                    |
|     status dropdown | type dropdown | search |
|   </form>                                    |
|   #archive-results                           |
|     .table-wrapper > .data-table             |
+---------------------------------------------+
```

The filter controls use `.form-group` wrappers inside a `.filter-controls` container. The form uses htmx to swap results on filter change (`hx-trigger="change, keyup changed delay:300ms"`).

---

## Settings Page

Single-column layout with multiple `.settings-section` groups inside one `.section`.

```
+---------------------------------------------+
| .section: Settings                           |
|   .settings-section: API Endpoint (readonly) |
|   .settings-section: Service Credentials     |
|     separate <form> for registration code    |
|   .settings-section: Player ID Hasher        |
|   .settings-section: Use Players DB          |
|   [Save Settings] button                     |
+---------------------------------------------+
```

The Service Credentials section has its own form with htmx partial swap. The Player ID Hasher and Use Players DB sections are in a single form with a standard POST action.

---

## Players DB Page

Simple single-column layout with one card inside a section.

```
+---------------------------------------------+
| .section: Players Database                   |
|   .card                                      |
|     entry count or "disabled" message        |
|     [Download] button or link to settings    |
+---------------------------------------------+
```

---

## Responsive Breakpoints

### Tablet (max-width: 1024px)

- `--max-width` overridden to `100%` (container fills viewport)
- `.dashboard-grid` collapses to single column

### Mobile (max-width: 768px)

- `.dashboard-grid` -- single column
- `.navbar .container` -- stacks vertically (brand above nav links)
- `.navbar-nav` -- wraps and centers
- `.data-table` -- becomes `display: block` with `overflow-x: auto` for horizontal scrolling
- `.btn` -- increased padding (`--space-sm` / `--space-lg`) and 44px minimum height for touch targets
- `.drop-zone` -- reduced padding to `--space-lg`
- `.modal-content` -- 95% width, `--space-lg` padding

---

## Maximum Width Constraints

| Element | Max Width |
|---|---|
| `.container` (general content) | 1200px (`--max-width`) |
| `.login-card` | 400px |
| `.setup-wizard` | 600px |
| `.modal-content` | 500px (90% width) |

---

## Spacing Conventions

Consistent spacing is applied through sections and components:

| Context | Spacing |
|---|---|
| Page content top/bottom padding | `--space-xl` (2rem) |
| Between sections | `--space-xl` (2rem) via `.section` margin-bottom |
| Card internal padding | `--space-lg` (1.5rem) |
| Between card bottom margin | `--space-lg` (1.5rem) |
| Form group bottom margin | `--space-md` (1rem) |
| Section title bottom margin | `--space-md` (1rem) |
| Table cell padding | `--space-sm` / `--space-md` |
| Dashboard grid gap | `--space-lg` (1.5rem) |
| Navbar padding | `--space-sm` / `--space-md` |
| Container horizontal padding | `--space-md` (1rem) |

---

## htmx Integration Notes

The layout includes these scripts on every authenticated page:

```html
<script src="/vendor/htmx.min.js"></script>
<script src="/js/sse.js"></script>
<script src="/js/app.js"></script>
```

The dashboard uses SSE (Server-Sent Events) via the htmx SSE extension:

```html
<div hx-ext="sse" sse-connect="/events">
  <div id="queued-table-body" sse-swap="queued">...</div>
  <div id="processing-body" sse-swap="processing">...</div>
  <div id="uploading-table-body" sse-swap="uploading">...</div>
  <div id="finished-table-body" sse-swap="recently-finished">...</div>
</div>
```

Each SSE event name (`queued`, `processing`, `uploading`, `recently-finished`) maps to a div that gets its innerHTML replaced with server-rendered HTML fragments.

The login page and setup wizard only load `htmx.min.js` (no SSE or app.js).
