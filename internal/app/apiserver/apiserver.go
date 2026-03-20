package apiserver

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/AVGsync/study_flow_api/internal/infrastructure/cache/rediscache"
	"github.com/AVGsync/study_flow_api/internal/infrastructure/security"
	"github.com/AVGsync/study_flow_api/internal/repository/postgres"
	"github.com/AVGsync/study_flow_api/internal/service"
	"github.com/AVGsync/study_flow_api/internal/transport/http/handler"
	"github.com/AVGsync/study_flow_api/internal/transport/http/middleware"
	"github.com/AVGsync/study_flow_api/internal/transport/ws"
	"github.com/go-chi/chi/v5"
)

type APIServer struct {
	config *Config
	logger *slog.Logger
	router *chi.Mux
	db     *postgres.DB
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
	hub := ws.NewHub()
	go hub.Run()

	userCache := rediscache.NewUserCache(s.config.RedisAddr, s.config.CacheTTL)
	userRepo := s.db.User()

	// Собираем зависимости слоями: transport -> service -> repository/infrastructure.
	userService := service.NewUserService(userRepo, security.NewBcryptHasher(), userCache)
	userHandler := handler.NewUserHandler(userService, security.NewValidator())
	chatHandler := handler.NewChatHandler(hub)

	authMW := middleware.NewMiddleware([]byte(s.config.JWTSecret), userService)

	s.router.Route("/api", func(r chi.Router) {
		r.Use(authMW.Auth)
		r.Get("/ws", chatHandler.ServeWS())
		r.Get("/user", userHandler.UserByID())
		r.Patch("/user", userHandler.Update())
		r.Patch("/user/change-password", userHandler.ChangePassword())

		r.Route("/admin", func(r chi.Router) {
			r.Use(authMW.Admin)
			r.Get("/user", userHandler.UserByID())
			r.Patch("/user", userHandler.Update())
			r.Patch("/user/change-password", userHandler.ChangePassword())
		})
	})
}

func (s *APIServer) configureDB() error {
	db := postgres.New(s.config.DB)
	if err := db.Open(); err != nil {
		return err
	}

	s.db = db
	return nil
}
