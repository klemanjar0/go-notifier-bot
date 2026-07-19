package health

import (
	"context"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/klemanjar0/go-notifier-bot/internal/logger"
)

type Server struct {
	srv *http.Server
	log *zap.Logger
}

func NewServer(addr string) *Server {
	mux := http.NewServeMux()
	s := &Server{log: logger.Named("health")}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	s.srv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return s
}

func (s *Server) Start() {
	s.log.Info("health server listening", zap.String("addr", s.srv.Addr))
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.log.Error("health server failed", zap.Error(err))
	}
}

func (s *Server) Shutdown(ctx context.Context) {
	if err := s.srv.Shutdown(ctx); err != nil {
		s.log.Error("health server shutdown failed", zap.Error(err))
	}
}
