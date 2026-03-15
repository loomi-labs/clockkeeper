package web

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1/clockkeeperv1connect"
)

// Server is the HTTP server that serves the API and frontend.
type Server struct {
	config     *Config
	httpServer *http.Server
}

// NewServer creates a new web server with all services wired.
func NewServer(config *Config, db *ent.Client, staticFiles fs.FS) *Server {
	auth := NewAuthInterceptor(config.JWTSecretKey)

	handler := &ClockKeeperServiceHandler{
		config: config,
		db:     db,
		auth:   auth,
	}

	mux := http.NewServeMux()

	// ConnectRPC API with auth interceptor.
	rpcPath, rpcHandler := clockkeeperv1connect.NewClockKeeperServiceHandler(
		handler,
		connect.WithInterceptors(auth),
	)
	mux.Handle(rpcPath, rpcHandler)

	// Svelte SPA (catch-all with fallback to index.html for client-side routing).
	if staticFiles != nil {
		mux.Handle("/", spaFileServer(staticFiles))
	}

	return &Server{
		config: config,
		httpServer: &http.Server{
			Addr:    config.Listen,
			Handler: mux,
		},
	}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	slog.Info("starting web server", "listen", s.config.Listen)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
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
