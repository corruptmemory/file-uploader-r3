package server

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/corruptmemory/file-uploader-r3/internal/app"
	"github.com/corruptmemory/file-uploader-r3/internal/auth"
	"github.com/corruptmemory/file-uploader-r3/internal/csv"
	"github.com/corruptmemory/file-uploader-r3/internal/server/pages"
	"github.com/corruptmemory/file-uploader-r3/internal/setup"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// rateLimitCmd is a query to check if an IP is allowed.
type rateLimitCmd struct {
	ip     string
	result chan<- bool
}

// rateLimiter tracks request timestamps per IP for simple rate limiting.
// A single goroutine owns the mutable state; callers communicate via channels.
type rateLimiter struct {
	cmdCh   chan rateLimitCmd
	quit    chan struct{}
	maxReqs int
	window  time.Duration
}

// newRateLimiter creates a rate limiter allowing maxReqs requests per window per IP
// and starts the actor goroutine.
func newRateLimiter(maxReqs int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		cmdCh:   make(chan rateLimitCmd),
		quit:    make(chan struct{}),
		maxReqs: maxReqs,
		window:  window,
	}
	go rl.run()
	return rl
}

func (rl *rateLimiter) run() {
	attempts := make(map[string][]time.Time)
	for {
		select {
		case cmd := <-rl.cmdCh:
			now := time.Now()
			cutoff := now.Add(-rl.window)

			// Evict old entries for this IP
			times := attempts[cmd.ip]
			valid := times[:0]
			for _, t := range times {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}

			if len(valid) >= rl.maxReqs {
				attempts[cmd.ip] = valid
				cmd.result <- false
				continue
			}

			attempts[cmd.ip] = append(valid, now)

			// Periodically clean up other IPs
			if len(attempts) > 100 {
				for k, v := range attempts {
					filtered := v[:0]
					for _, t := range v {
						if t.After(cutoff) {
							filtered = append(filtered, t)
						}
					}
					if len(filtered) == 0 {
						delete(attempts, k)
					} else {
						attempts[k] = filtered
					}
				}
			}

			cmd.result <- true
		case <-rl.quit:
			return
		}
	}
}

// allow checks whether the given IP is within the rate limit. It records the
// attempt and evicts stale entries.
func (rl *rateLimiter) allow(ip string) bool {
	ch := make(chan bool, 1)
	rl.cmdCh <- rateLimitCmd{ip: ip, result: ch}
	return <-ch
}

// close signals the actor goroutine to stop.
func (rl *rateLimiter) close() {
	close(rl.quit)
}

// clientIP extracts the client IP from a request, stripping the port.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// maxUploadSize is the maximum allowed upload size (50 MB).
const maxUploadSize = 50 << 20 // 50 MB

// WebApp registers all HTTP routes and handles requests.
type WebApp struct {
	app              *app.Application
	authProvider     app.AuthProvider
	signingKey       []byte
	uploadDir        string
	version          string
	prefix           string
	tlsEnabled       bool
	tokenBlacklist   *auth.TokenBlacklist
	setupRateLimiter *rateLimiter
	loginRateLimiter *rateLimiter
}

// NewWebApp creates a WebApp and registers routes on a new chi.Router.
// staticFS should be an fs.FS rooted at the directory containing js/, css/, img/.
func NewWebApp(application *app.Application, authProvider app.AuthProvider, signingKey []byte, uploadDir, version, prefix string, staticFS fs.FS, tlsEnabled bool) (*WebApp, chi.Router) {
	wa := &WebApp{
		app:              application,
		authProvider:     authProvider,
		signingKey:       signingKey,
		uploadDir:        uploadDir,
		version:          version,
		prefix:           prefix,
		tlsEnabled:       tlsEnabled,
		tokenBlacklist:   auth.NewTokenBlacklist(),
		setupRateLimiter: newRateLimiter(10, 1*time.Minute),
		loginRateLimiter: newRateLimiter(5, 1*time.Minute),
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	// Mount routes under prefix
	if prefix != "" && prefix != "/" {
		r.Route(prefix, func(sub chi.Router) {
			wa.registerRoutes(sub, staticFS)
		})
	} else {
		wa.registerRoutes(r, staticFS)
	}

	return wa, r
}

// securityHeaders is a chi middleware that sets security-related HTTP response headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// noDirectoryListing wraps an http.Handler to return 404 for directory requests.
func noDirectoryListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (wa *WebApp) registerRoutes(r chi.Router, staticFS fs.FS) {
	// Static assets — no auth, any state; directory listing disabled
	fileServer := noDirectoryListing(http.FileServer(http.FS(staticFS)))
	r.Handle("/js/*", fileServer)
	r.Handle("/css/*", fileServer)
	r.Handle("/img/*", fileServer)
	r.Handle("/vendor/*", fileServer)
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		f, err := staticFS.Open("img/favicon.ico")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "image/x-icon")
		http.ServeContent(w, r, "favicon.ico", time.Time{}, f.(io.ReadSeeker))
	})

	// Health — no auth, any state
	r.Get("/health", wa.handleHealth)

	// Setup routes — no auth, SetupApp state
	r.Group(func(sub chi.Router) {
		sub.Get("/setup", wa.withSetupApp(wa.handleSetupGet))
		sub.Post("/setup/{action}", wa.withSetupApp(wa.handleSetupPost))
	})

	// Login/logout — no auth, RunningApp state
	r.Group(func(sub chi.Router) {
		sub.Get("/login", wa.withRunningState(wa.withStateOptionalSession(wa.handleLoginGet)))
		sub.Post("/login", wa.withRunningState(wa.handleLoginPost))
		sub.Post("/logout", wa.withRunningState(wa.handleLogout))
	})

	// Authenticated routes — require session + RunningApp
	r.Group(func(sub chi.Router) {
		sub.Get("/", wa.withRunningState(wa.withStateAndSession(wa.handleDashboard)))
		sub.Post("/upload", wa.withRunningState(wa.withStateAndSession(wa.handleUpload)))
		sub.Get("/events", wa.withRunningState(wa.withStateAndSession(wa.handleEvents)))
		sub.Get("/api/extend", wa.withRunningState(wa.withStateAndSession(wa.handleExtendSession)))
		sub.Get("/failure-details/{record-id}", wa.withRunningState(wa.withStateAndSession(wa.handleFailureDetails)))
		sub.Get("/settings", wa.withRunningState(wa.withStateAndSession(wa.handleSettingsGet)))
		sub.Post("/settings", wa.withRunningState(wa.withStateAndSession(wa.handleSettingsPost)))
		sub.Post("/settings/registration", wa.withRunningState(wa.withStateAndSession(wa.handleRegistrationCode)))
		sub.Get("/players-db", wa.withRunningState(wa.withStateAndSession(wa.handlePlayersDB)))
		sub.Get("/download-players-db", wa.withRunningState(wa.withStateAndSession(wa.handleDownloadPlayersDB)))
		sub.Get("/archived", wa.withRunningState(wa.withStateAndSession(wa.handleArchived)))
		sub.Post("/search-archived", wa.withRunningState(wa.withStateAndSession(wa.handleSearchArchived)))
	})
}

// --- Session Middleware ---

// withStateAndSession validates the session JWT. Invalid/expired redirects to /login.
func (wa *WebApp) withStateAndSession(handler func(http.ResponseWriter, *http.Request, *auth.JWTClaims)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := wa.parseClaims(r)
		if claims == nil {
			auth.ClearSessionCookies(w, wa.cookiePath(), wa.tlsEnabled)
			loginPath := "/login"
			if wa.prefix != "" && wa.prefix != "/" {
				loginPath = wa.prefix + loginPath
			}
			http.Redirect(w, r, loginPath, http.StatusSeeOther)
			return
		}
		handler(w, r, claims)
	}
}

// withStateOptionalSession parses the session but does not redirect on failure.
func (wa *WebApp) withStateOptionalSession(handler func(http.ResponseWriter, *http.Request, *auth.JWTClaims)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := wa.parseClaims(r)
		handler(w, r, claims)
	}
}

func (wa *WebApp) parseClaims(r *http.Request) *auth.JWTClaims {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	claims, err := auth.ParseToken(cookie.Value, wa.signingKey)
	if err != nil {
		return nil
	}
	// Check if the token has been revoked
	if claims.JTI != "" && wa.tokenBlacklist.IsRevoked(claims.JTI) {
		return nil
	}
	return claims
}

func (wa *WebApp) cookiePath() string {
	if wa.prefix != "" && wa.prefix != "/" {
		return wa.prefix
	}
	return "/"
}

// --- State-Aware Routing Helpers ---

// withRunningState checks that the app is in RunningApp state.
func (wa *WebApp) withRunningState(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := wa.app.GetState()
		if err != nil {
			http.Error(w, "application unavailable", http.StatusServiceUnavailable)
			return
		}

		switch s := state.(type) {
		case app.RunningApp:
			handler(w, r)
		case app.SetupApp:
			setupPath := "/setup"
			if wa.prefix != "" && wa.prefix != "/" {
				setupPath = wa.prefix + setupPath
			}
			http.Redirect(w, r, setupPath, http.StatusSeeOther)
		case app.ErrorApp:
			log.Printf("Application error: %v", s.GetError())
			http.Error(w, "Application error — please contact your administrator", http.StatusInternalServerError)
		default:
			http.Error(w, "unknown application state", http.StatusInternalServerError)
		}
	}
}

// withSetupApp checks that the app is in SetupApp state.
func (wa *WebApp) withSetupApp(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := wa.app.GetState()
		if err != nil {
			http.Error(w, "application unavailable", http.StatusServiceUnavailable)
			return
		}

		switch s := state.(type) {
		case app.SetupApp:
			handler(w, r)
		case app.RunningApp:
			rootPath := "/"
			if wa.prefix != "" && wa.prefix != "/" {
				rootPath = wa.prefix + "/"
			}
			http.Redirect(w, r, rootPath, http.StatusSeeOther)
		case app.ErrorApp:
			log.Printf("Application error: %v", s.GetError())
			http.Error(w, "Application error — please contact your administrator", http.StatusInternalServerError)
		default:
			http.Error(w, "unknown application state", http.StatusInternalServerError)
		}
	}
}

// --- Handlers ---

func (wa *WebApp) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func (wa *WebApp) getRunningApp() (app.RunningApp, error) {
	state, err := wa.app.GetState()
	if err != nil {
		return nil, err
	}
	ra, ok := state.(app.RunningApp)
	if !ok {
		return nil, fmt.Errorf("not in running state")
	}
	return ra, nil
}

func (wa *WebApp) handleDashboard(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	ra, err := wa.getRunningApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	runState, err := ra.GetState()
	if err != nil {
		http.Error(w, "failed to get state", http.StatusInternalServerError)
		return
	}

	var processingState app.CSVProcessingState
	if runState != nil {
		processingState = runState.DataProcessing
	}

	w.Header().Set("Content-Type", "text/html")
	component := pages.DashboardFullPage(wa.prefix, processingState)
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("dashboard render error: %v", err)
	}
}

func (wa *WebApp) handleLoginGet(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	// If user already has a valid session, redirect to dashboard
	if claims != nil {
		rootPath := "/"
		if wa.prefix != "" && wa.prefix != "/" {
			rootPath = wa.prefix + "/"
		}
		http.Redirect(w, r, rootPath, http.StatusSeeOther)
		return
	}

	ra, err := wa.getRunningApp()
	needsMFA := false
	if err == nil {
		needsMFA, _ = ra.MFARequired()
	}

	w.Header().Set("Content-Type", "text/html")
	component := pages.LoginPageFull(wa.prefix, pages.LoginFormData{NeedsMFA: needsMFA})
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("login page render error: %v", err)
	}
}

func (wa *WebApp) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if !wa.loginRateLimiter.allow(clientIP(r)) {
		http.Error(w, "too many login attempts — please wait and try again", http.StatusTooManyRequests)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	mfaToken := r.FormValue("mfa_token")

	ra, raErr := wa.getRunningApp()
	needsMFA := false
	if raErr == nil {
		needsMFA, _ = ra.MFARequired()
	}

	if username == "" || password == "" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		component := pages.LoginForm(wa.prefix, pages.LoginFormData{
			Username: html.EscapeString(username),
			NeedsMFA: needsMFA,
			Error:    "Username and password are required",
		})
		component.Render(r.Context(), w)
		return
	}

	// Authenticate via the AuthProvider
	sessionToken, err := wa.authProvider.Login(username, password, mfaToken)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
		component := pages.LoginForm(wa.prefix, pages.LoginFormData{
			Username: html.EscapeString(username),
			NeedsMFA: needsMFA,
			Error:    "Invalid credentials",
		})
		component.Render(r.Context(), w)
		return
	}

	claims := auth.NewClaims(sessionToken.Username, sessionToken.OrgID)

	tokenStr, err := auth.CreateToken(claims, wa.signingKey)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	expiry := time.Unix(claims.Exp, 0)
	auth.SetSessionCookies(w, tokenStr, expiry, wa.cookiePath(), wa.tlsEnabled)

	rootPath := "/"
	if wa.prefix != "" && wa.prefix != "/" {
		rootPath = wa.prefix + "/"
	}

	// If htmx request, use HX-Redirect header
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", rootPath)
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, rootPath, http.StatusSeeOther)
}

func (wa *WebApp) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Revoke the current token before clearing cookies
	cookie, err := r.Cookie("session")
	if err == nil {
		claims, parseErr := auth.ParseToken(cookie.Value, wa.signingKey)
		if parseErr == nil && claims.JTI != "" {
			wa.tokenBlacklist.Revoke(claims.JTI, time.Unix(claims.Exp, 0))
		}
	}

	auth.ClearSessionCookies(w, wa.cookiePath(), wa.tlsEnabled)
	loginPath := "/login"
	if wa.prefix != "" && wa.prefix != "/" {
		loginPath = wa.prefix + loginPath
	}
	http.Redirect(w, r, loginPath, http.StatusSeeOther)
}

func (wa *WebApp) handleUpload(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	// Enforce 50MB limit
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "file too large (max 50MB)", http.StatusRequestEntityTooLarge)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="alert alert-danger">No files selected.</div>`)
		return
	}

	ra, err := wa.getRunningApp()
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="alert alert-danger">Application unavailable.</div>`)
		return
	}

	var uploaded []string
	var errors []string

	for _, fh := range files {
		escapedName := html.EscapeString(fh.Filename)
		if !strings.HasSuffix(strings.ToLower(fh.Filename), ".csv") {
			errors = append(errors, fmt.Sprintf("%s: not a CSV file", escapedName))
			continue
		}

		src, err := fh.Open()
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: failed to open", escapedName))
			continue
		}

		// Sanitize filename and create unique local path
		sanitized := sanitizeFilename(fh.Filename)
		localName := fmt.Sprintf("%d-%s", time.Now().UnixNano(), sanitized)
		localPath := filepath.Join(wa.uploadDir, localName)

		dst, err := os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			src.Close()
			errors = append(errors, fmt.Sprintf("%s: failed to save", escapedName))
			continue
		}

		_, copyErr := io.Copy(dst, src)
		src.Close()
		dst.Close()

		if copyErr != nil {
			os.Remove(localPath)
			errors = append(errors, fmt.Sprintf("%s: failed to save", escapedName))
			continue
		}

		if err := ra.ProcessUploadedCSVFile(claims.Username, fh.Filename, localPath); err != nil {
			os.Remove(localPath)
			errors = append(errors, fmt.Sprintf("%s: %s", escapedName, html.EscapeString(err.Error())))
			continue
		}

		uploaded = append(uploaded, escapedName)
	}

	w.Header().Set("Content-Type", "text/html")
	if len(errors) > 0 {
		fmt.Fprintf(w, `<div class="alert alert-danger">Errors: %s</div>`, strings.Join(errors, "; "))
	}
	if len(uploaded) > 0 {
		fmt.Fprintf(w, `<div class="alert alert-success">Uploaded: %s</div>`, strings.Join(uploaded, ", "))
	}
}

// sanitizeFilename removes path separators and other unsafe characters from a filename.
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." {
		name = "upload.csv"
	}
	return name
}

func (wa *WebApp) handleEvents(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ra, err := wa.getRunningApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	sub, err := ra.Subscribe()
	if err != nil || sub == nil {
		http.Error(w, "failed to subscribe", http.StatusInternalServerError)
		return
	}
	defer ra.Unsubscribe(sub.ID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-sub.Events:
			if !ok {
				return
			}
			wa.writeSSEEvent(w, ctx, "queued", pages.QueuedTable(event.State.QueuedFiles))
			wa.writeSSEEvent(w, ctx, "processing", pages.ProcessingDisplay(event.State.ProcessingFile))
			wa.writeSSEEvent(w, ctx, "uploading", pages.UploadingTable(event.State.UploadingFiles))
			wa.writeSSEEvent(w, ctx, "recently-finished", pages.RecentlyFinishedTable(wa.prefix, event.State.FinishedFiles))
			flusher.Flush()
		}
	}
}

// writeSSEEvent renders a templ component to a single-line string and writes it as a named SSE event.
func (wa *WebApp) writeSSEEvent(w http.ResponseWriter, ctx context.Context, eventName string, component templ.Component) {
	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		log.Printf("SSE render error for %s: %v", eventName, err)
		return
	}
	line := strings.ReplaceAll(buf.String(), "\n", "")
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, line)
}

func (wa *WebApp) handleExtendSession(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	expiresIn := time.Until(time.Unix(claims.Exp, 0))

	if expiresIn > auth.ExtensionWindow {
		// Outside window — no action needed
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
		return
	}

	// Within window — issue new token
	newClaims := auth.NewClaims(claims.Username, claims.OrgID)
	tokenStr, err := auth.CreateToken(newClaims, wa.signingKey)
	if err != nil {
		http.Error(w, "failed to extend session", http.StatusInternalServerError)
		return
	}

	expiry := time.Unix(newClaims.Exp, 0)
	auth.SetSessionCookies(w, tokenStr, expiry, wa.cookiePath(), wa.tlsEnabled)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "extended")
}

func (wa *WebApp) handleFailureDetails(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	recordID := chi.URLParam(r, "record-id")
	if recordID == "" {
		http.Error(w, "missing record ID", http.StatusBadRequest)
		return
	}

	ra, err := wa.getRunningApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	details, err := ra.GetFinishedDetails(recordID)
	if err != nil {
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	component := pages.FailureDetailsContent(details)
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("failure details render error: %v", err)
	}
}

func (wa *WebApp) handleSettingsGet(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	ra, err := wa.getRunningApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	config, err := ra.GetConfig()
	if err != nil {
		http.Error(w, "failed to get config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	component := pages.SettingsPage(wa.prefix, pages.SettingsData{Config: config})
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("settings render error: %v", err)
	}
}

func (wa *WebApp) handleSettingsPost(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	ra, err := wa.getRunningApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	config, err := ra.GetConfig()
	if err != nil {
		http.Error(w, "failed to get config", http.StatusInternalServerError)
		return
	}

	// Apply settable values from form
	pepper := r.FormValue("pepper")
	usePlayersDB := r.FormValue("use_players_db")

	updated := config.
		WithOrgPlayerIDPepper(pepper).
		WithOrgPlayerIDHash("argon2").
		WithUsePlayersDB(usePlayersDB)

	// Validate
	validErr := updated.ValidateSettingsPageValues(nil)
	if validErr != nil {
		configErrors, ok := validErr.(*app.ApplicationConfigErrors)
		if !ok {
			http.Error(w, "validation error", http.StatusInternalServerError)
			return
		}
		// Filter to only settable field errors (pepper, use_players_db)
		w.Header().Set("Content-Type", "text/html")
		component := pages.SettingsPage(wa.prefix, pages.SettingsData{
			Config: updated,
			Errors: configErrors,
		})
		if err := component.Render(r.Context(), w); err != nil {
			log.Printf("settings render error: %v", err)
		}
		return
	}

	// Save config
	if err := ra.UpdateConfig(updated); err != nil {
		http.Error(w, "failed to save settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	component := pages.SettingsPage(wa.prefix, pages.SettingsData{
		Config:     updated,
		SuccessMsg: "Settings saved successfully.",
	})
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("settings render error: %v", err)
	}
}

func (wa *WebApp) handleRegistrationCode(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	ra, err := wa.getRunningApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	config, err := ra.GetConfig()
	if err != nil {
		http.Error(w, "failed to get config", http.StatusInternalServerError)
		return
	}

	code := r.FormValue("registration_code")
	if code == "" {
		w.Header().Set("Content-Type", "text/html")
		component := pages.RegCodeResult(false, "Registration code is required")
		component.Render(r.Context(), w)
		return
	}

	_, consumeErr := wa.authProvider.ConsumeRegistrationCode(config.Endpoint, code)
	if consumeErr != nil {
		w.Header().Set("Content-Type", "text/html")
		component := pages.RegCodeResult(false, "Failed to consume registration code: "+html.EscapeString(consumeErr.Error()))
		component.Render(r.Context(), w)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	component := pages.RegCodeResult(true, "Registration code accepted successfully.")
	component.Render(r.Context(), w)
}

func (wa *WebApp) handlePlayersDB(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	ra, err := wa.getRunningApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	runState, err := ra.GetState()
	if err != nil {
		http.Error(w, "failed to get state", http.StatusInternalServerError)
		return
	}

	dbState := app.PlayersDBState{}
	if runState != nil {
		dbState = runState.PlayersDB
	}

	w.Header().Set("Content-Type", "text/html")
	component := pages.PlayersDBPage(wa.prefix, dbState)
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("players-db render error: %v", err)
	}
}

func (wa *WebApp) handleDownloadPlayersDB(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	ra, err := wa.getRunningApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	config, err := ra.GetConfig()
	if err != nil {
		http.Error(w, "failed to get config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="players.db"`)
	w.Header().Set("Content-Type", "application/octet-stream")

	if err := ra.DownloadPlayersDB(config.OrgPlayerIDHash, config.OrgPlayerIDPepper, w); err != nil {
		log.Printf("download players DB error: %v", err)
	}
}

func (wa *WebApp) handleArchived(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	ra, err := wa.getRunningApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	files, err := ra.SearchFinished(app.FinishedStatusAll, nil, "")
	if err != nil {
		http.Error(w, "failed to get archived files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	component := pages.ArchivedPage(wa.prefix, files)
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("archived render error: %v", err)
	}
}

func (wa *WebApp) handleSearchArchived(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	ra, err := wa.getRunningApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	statusStr := r.FormValue("status")
	status := app.FinishedStatus(statusStr)

	var csvTypes []csv.CSVType
	csvTypeStr := r.FormValue("csv_type")
	if csvTypeStr != "" {
		ct, err := csv.CSVTypeFromSlug(csvTypeStr)
		if err == nil {
			csvTypes = []csv.CSVType{ct}
		}
	}

	search := r.FormValue("search")

	files, err := ra.SearchFinished(status, csvTypes, search)
	if err != nil {
		http.Error(w, "failed to search archived files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	component := pages.ArchiveResultsTable(wa.prefix, files)
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("search archived render error: %v", err)
	}
}

func (wa *WebApp) getSetupApp() (app.SetupApp, error) {
	state, err := wa.app.GetState()
	if err != nil {
		return nil, err
	}
	sa, ok := state.(app.SetupApp)
	if !ok {
		return nil, fmt.Errorf("not in setup state")
	}
	return sa, nil
}

func (wa *WebApp) handleSetupGet(w http.ResponseWriter, r *http.Request) {
	sa, err := wa.getSetupApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	info, err := sa.GetCurrentState()
	if err != nil {
		http.Error(w, "failed to get setup state", http.StatusInternalServerError)
		return
	}

	stepInfo, ok := info.(*setup.StepInfo)
	if !ok {
		http.Error(w, "invalid step info", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	component := pages.SetupPage(stepInfo, wa.prefix)
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("setup page render error: %v", err)
	}
}

func (wa *WebApp) handleSetupPost(w http.ResponseWriter, r *http.Request) {
	// CSRF protection: require HX-Request header (set automatically by htmx).
	// Browsers do not send custom headers on cross-origin form submissions.
	if r.Header.Get("HX-Request") != "true" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Rate limiting on setup POST endpoints
	if !wa.setupRateLimiter.allow(clientIP(r)) {
		http.Error(w, "too many requests — please wait and try again", http.StatusTooManyRequests)
		return
	}

	action := chi.URLParam(r, "action")

	sa, err := wa.getSetupApp()
	if err != nil {
		http.Error(w, "application unavailable", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	var info app.SetupStepInfo

	switch action {
	case "next":
		stepStr := r.FormValue("current_step")
		step, parseErr := strconv.Atoi(stepStr)
		if parseErr != nil {
			http.Error(w, "invalid step", http.StatusBadRequest)
			return
		}

		switch app.SetupStepNumber(step) {
		case app.StepWelcome:
			info, err = sa.GetServiceEndpoint()
		case app.StepEndpoint:
			endpoint := r.FormValue("endpoint")
			env := r.FormValue("environment")
			info, err = sa.SetServiceEndpoint(endpoint, env)
		case app.StepServiceCredentials:
			code := r.FormValue("registration_code")
			info, err = sa.UseRegistrationCode(code)
		case app.StepPlayerIDHasher:
			pepper := r.FormValue("pepper")
			hashAlg := r.FormValue("hash_algorithm")
			if hashAlg == "" {
				hashAlg = "argon2"
			}
			info, err = sa.SetPlayerIDHasher(pepper, hashAlg)
		case app.StepUsePlayersDB:
			useDB := r.FormValue("use_players_db") == "true"
			info, err = sa.SetUsePlayerDB(useDB)
		default:
			http.Error(w, "invalid step", http.StatusBadRequest)
			return
		}

	case "back":
		stepStr := r.FormValue("current_step")
		step, parseErr := strconv.Atoi(stepStr)
		if parseErr != nil {
			http.Error(w, "invalid step", http.StatusBadRequest)
			return
		}
		info, err = sa.GoBackFrom(app.SetupStepNumber(step))

	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "setup error", http.StatusInternalServerError)
		return
	}

	stepInfo, ok := info.(*setup.StepInfo)
	if !ok {
		http.Error(w, "invalid step info", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	component := pages.SetupStepContent(stepInfo, wa.prefix)
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("setup step render error: %v", err)
	}
}
