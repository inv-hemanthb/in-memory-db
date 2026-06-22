package api

import (
	"context"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"time"

	apidb "github.com/inv-hemanthb/in-memory-db/internal/api/db"
	"github.com/inv-hemanthb/in-memory-db/internal/api/kvclient"
	"github.com/inv-hemanthb/in-memory-db/internal/logger"
)

type Server struct {
	service   *ItemService
	metrics   *Metrics
	log       *logger.Logger
	mux       *http.ServeMux
	templates *template.Template
}

func NewServer(store *apidb.Store, kv *kvclient.Client, log *logger.Logger) *Server {
	templates, err := loadTemplates()
	if err != nil {
		log.Error("load templates: %v", err)
		panic(err)
	}

	s := &Server{
		service:   NewItemService(store, kv),
		metrics:   NewMetrics(),
		log:       log,
		mux:       http.NewServeMux(),
		templates: templates,
	}

	staticPath, err := staticDir()
	if err != nil {
		log.Error("static dir: %v", err)
		panic(err)
	}

	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticPath))))
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("POST /items", s.handleCreate)
	s.mux.HandleFunc("GET /items/read", s.handleRead)
	s.mux.HandleFunc("POST /items/update", s.handleUpdate)
	s.mux.HandleFunc("POST /items/delete", s.handleDelete)
	s.mux.HandleFunc("POST /cache/clear", s.handleClearCache)

	return s
}

func (s *Server) Run(ctx context.Context, addr string) error {
	httpServer := &http.Server{
		Addr:    addr,
		Handler: s.mux,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("HTTP server started on %s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		s.log.Info("HTTP server stopped")
		return nil
	case err := <-errCh:
		return err
	}
}
