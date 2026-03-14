package server

import (
	"fmt"
	"html"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/corruptmemory/file-uploader-r3/internal/app"
	"github.com/corruptmemory/file-uploader-r3/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// maxUploadSize is the maximum allowed upload size (50 MB).
const maxUploadSize = 50 << 20 // 50 MB

// WebApp registers all HTTP routes and handles requests.
type WebApp struct {
	app            *app.Application
	authProvider   app.AuthProvider
	signingKey     []byte
	uploadDir      string
	version        string
	prefix         string
	tlsEnabled     bool
	tokenBlacklist *auth.TokenBlacklist
}

// NewWebApp creates a WebApp and registers routes on a new chi.Router.
// staticFS should be an fs.FS rooted at the directory containing js/, css/, img/.
func NewWebApp(application *app.Application, authProvider app.AuthProvider, signingKey []byte, uploadDir, version, prefix string, staticFS fs.FS, tlsEnabled bool) (*WebApp, chi.Router) {
	wa := &WebApp{
		app:            application,
		authProvider:   authProvider,
		signingKey:     signingKey,
		uploadDir:      uploadDir,
		version:        version,
		prefix:         prefix,
		tlsEnabled:     tlsEnabled,
		tokenBlacklist: auth.NewTokenBlacklist(),
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
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (wa *WebApp) registerRoutes(r chi.Router, staticFS fs.FS) {
	// Static assets — no auth, any state
	fileServer := http.FileServer(http.FS(staticFS))
	r.Handle("/js/*", fileServer)
	r.Handle("/css/*", fileServer)
	r.Handle("/img/*", fileServer)
	r.Handle("/favicon.ico", fileServer)

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
		sub.Get("/logout", wa.withRunningState(wa.handleLogout))
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

func (wa *WebApp) handleDashboard(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<html><body><h1>Dashboard</h1><p>Welcome, %s</p></body></html>", html.EscapeString(claims.Username))
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
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<html><body><h1>Login</h1><form method="POST"><input name="username" placeholder="Username"><input name="password" type="password" placeholder="Password"><button type="submit">Login</button></form></body></html>`)
}

func (wa *WebApp) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	mfaToken := r.FormValue("mfa_token")

	if username == "" || password == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "username and password required")
		return
	}

	// Authenticate via the AuthProvider
	sessionToken, err := wa.authProvider.Login(username, password, mfaToken)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "invalid credentials")
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

	// Placeholder — actual file handling comes in later specs
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "upload received")
}

func (wa *WebApp) handleEvents(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	// SSE placeholder
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fmt.Fprint(w, "data: {\"status\":\"connected\"}\n\n")
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
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, "<div>Failure details placeholder</div>")
}

func (wa *WebApp) handleSettingsGet(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, "<html><body><h1>Settings</h1></body></html>")
}

func (wa *WebApp) handleSettingsPost(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, "<html><body><h1>Settings updated</h1></body></html>")
}

func (wa *WebApp) handlePlayersDB(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, "<html><body><h1>Players Database</h1></body></html>")
}

func (wa *WebApp) handleDownloadPlayersDB(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "download placeholder")
}

func (wa *WebApp) handleArchived(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, "<html><body><h1>Archived Files</h1></body></html>")
}

func (wa *WebApp) handleSearchArchived(w http.ResponseWriter, r *http.Request, claims *auth.JWTClaims) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, "<html><body><h1>Search Results</h1></body></html>")
}

func (wa *WebApp) handleSetupGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, "<html><body><h1>Setup Wizard</h1></body></html>")
}

func (wa *WebApp) handleSetupPost(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<html><body><h1>Setup: %s</h1></body></html>", html.EscapeString(action))
}
