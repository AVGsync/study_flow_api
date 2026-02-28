package apiserver

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/AVGsync/study_flow_api/internal/database"
	"github.com/AVGsync/study_flow_api/internal/handlers"
	"github.com/AVGsync/study_flow_api/internal/auth"
	"github.com/go-chi/chi/v5"
)

type APIServer struct {
	config *Config
	logger *slog.Logger
	router *chi.Mux
	db     *database.DB
}

const (
	APIServerModeDebug = "debug"
)

func New(config *Config) *APIServer {

	logger := slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	)

	r := chi.NewRouter()

	return &APIServer{
		config: config,
		logger: logger,
		router: r,
	}
}

func (s *APIServer) Start() error {
	if err := s.configureLogger(); err != nil {
		return err
	}

	if err := s.configureDB(); err != nil {
		return err
	}

	s.configureRouter()

	s.logger.Info("starting api server")

	return http.ListenAndServe(s.config.BindAddr, s.router)
}

func (s *APIServer) configureLogger() error {
	var level slog.Level

	switch s.config.LogLevel {
	case APIServerModeDebug:
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	s.logger = slog.New(handler)
	slog.SetDefault(s.logger)

	return nil
}

func (s *APIServer) configureRouter() {
	userHandler := handlers.NewUserHandler(s.db.User())

	authMW := auth.NewMiddleware([]byte(s.config.JWTSecret), s.db.User())

	s.router.Route("/api", func(r chi.Router) {
		r.Use(authMW.Auth)
		r.Get("/user", userHandler.Me())
	})


}

func (s *APIServer) configureDB() error {
	db := database.New(s.config.DB)
	if err := db.Open(); err != nil {
		return err
	}

	s.db = db
	return nil
}
