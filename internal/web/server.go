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

// Server is the HTTP server that serves the API and frontend.
type Server struct {
	config      *Config
	httpServer  *http.Server
	cancelFunc  context.CancelFunc
	rateLimiter *RateLimitInterceptor
}

// NewServer creates a new web server with all services wired.
func NewServer(config *Config, db *ent.Client, registry *botc.Registry, staticFiles fs.FS, characterIcons fs.FS) *Server {
	auth := NewAuthInterceptor(config.JWTSecretKey)
	rateLimiter := NewRateLimitInterceptor(config.RateLimitAnon, config.RateLimitAuth)

	handler := &ClockKeeperServiceHandler{
		config:   config,
		db:       db,
		auth:     auth,
		registry: registry,
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
			Addr:           config.Listen,
			Handler:        securityHeaders(mux),
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   60 * time.Second,
			IdleTimeout:    120 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1 MB
		},
		cancelFunc:  cancel,
		rateLimiter: rateLimiter,
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
	return s.httpServer.Shutdown(ctx)
}

// securityHeaders wraps a handler with standard security response headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; font-src 'self'")
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
