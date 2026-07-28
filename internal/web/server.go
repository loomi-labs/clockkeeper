package web

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1/clockkeeperv1connect"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

// writeTimeout bounds how long a handler may take to write its response. It is
// applied per request rather than through http.Server.WriteTimeout, which would
// also kill long-lived streams.
const writeTimeout = 60 * time.Second

// watchTokenBagPath is the HTTP path of the WatchTokenBag stream. It comes from
// the generated procedure name, like the skipAuth entry for the same route, so
// the two cannot drift apart.
const watchTokenBagPath = clockkeeperv1connect.ClockKeeperServiceWatchTokenBagProcedure

// Server is the HTTP server that serves the API and frontend.
type Server struct {
	config      *Config
	httpServer  *http.Server
	cancelFunc  context.CancelFunc
	rateLimiter *RateLimitInterceptor
	hub         *TokenBagHub
}

// NewServer creates a new web server with all services wired.
func NewServer(config *Config, db *ent.Client, registry *botc.Registry, staticFiles fs.FS, characterIcons fs.FS) *Server {
	auth := NewAuthInterceptor(config.JWTSecretKey)
	rateLimiter := NewRateLimitInterceptor(config.RateLimitAnon, config.RateLimitAuth)
	hub := NewTokenBagHub()

	handler := &ClockKeeperServiceHandler{
		config:   config,
		db:       db,
		auth:     auth,
		registry: registry,
		hub:      hub,
	}

	mux := http.NewServeMux()

	// ConnectRPC API with auth and rate limit interceptors.
	rpcPath, rpcHandler := clockkeeperv1connect.NewClockKeeperServiceHandler(
		handler,
		connect.WithInterceptors(auth, rateLimiter),
	)
	mux.Handle(rpcPath, rpcHandler)

	// Character icon images.
	if characterIcons != nil {
		mux.Handle("/characters/", http.StripPrefix("/characters/", http.FileServer(http.FS(characterIcons))))
	}

	// Svelte SPA (catch-all with fallback to index.html for client-side routing).
	if staticFiles != nil {
		mux.Handle("/", spaFileServer(staticFiles))
	}

	ctx, cancel := context.WithCancel(context.Background())
	go startCleanup(ctx, db, config.AnonymousMaxAge)

	return &Server{
		config: config,
		httpServer: &http.Server{
			Addr:        config.Listen,
			Handler:     securityHeaders(perRequestWriteDeadline(mux)),
			ReadTimeout: 30 * time.Second,
			// No global write timeout — perRequestWriteDeadline applies it to
			// everything except the watch stream, which must stay open for hours.
			WriteTimeout:   0,
			IdleTimeout:    120 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1 MB
		},
		cancelFunc:  cancel,
		rateLimiter: rateLimiter,
		hub:         hub,
	}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	slog.Info("starting web server", "listen", s.config.Listen)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.cancelFunc()       // Stop cleanup goroutine.
	s.rateLimiter.Stop() // Stop rate limiter goroutine.
	// Before the HTTP shutdown: it waits for in-flight requests, and a watch
	// stream never finishes on its own.
	s.hub.Close()
	return s.httpServer.Shutdown(ctx)
}

// perRequestWriteDeadline gives every request the write deadline that
// http.Server.WriteTimeout used to enforce globally, except the token bag watch
// stream, whose liveness comes from its heartbeat and the client's context.
func perRequestWriteDeadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != watchTokenBagPath {
			// Errors mean the ResponseWriter has no deadline support; then the
			// request simply runs without one, as it would have anyway.
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(writeTimeout))
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeaders wraps a handler with standard security response headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// connect-src: the Spotify panel calls the Web API directly from the
		// browser with a backend-vended token. img-src: playlist cover art is
		// served from Spotify's CDNs.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https://*.scdn.co https://*.spotifycdn.com; connect-src 'self' https://api.spotify.com; font-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// spaFileServer serves static files from the given filesystem, falling back to
// index.html for paths that don't match a file (SPA client-side routing).
func spaFileServer(files fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}
		requestPath := strings.TrimPrefix(r.URL.Path, "/")
		f, err := files.Open(requestPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) &&
				path.Ext(requestPath) == "" &&
				strings.Contains(r.Header.Get("Accept"), "text/html") {
				r.URL.Path = "/"
				fileServer.ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
			return
		}
		_ = f.Close()
		fileServer.ServeHTTP(w, r)
	})
}
