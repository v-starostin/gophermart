package application

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	middleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/v-starostin/gophermart/internal/api"
	"github.com/v-starostin/gophermart/internal/config"
	"github.com/v-starostin/gophermart/internal/service"
	"github.com/v-starostin/gophermart/internal/storage"
)

type Server struct {
	srv    *http.Server
	logger *slog.Logger
}

func NewServer(l *slog.Logger, addr string) *Server {
	return &Server{
		srv:    &http.Server{Addr: addr},
		logger: l,
	}
}
func (s *Server) RegisterHandlers(cfg *config.Config, svc api.Service) {
	swagger, err := api.GetSwagger()
	if err != nil {
		s.logger.Info("'GetSwagger' error", slog.String("error", err.Error()))
		return
	}

	swagger.Servers = nil

	r := chi.NewRouter()
	r.Use(middleware.OapiRequestValidator(swagger))
	r.Use(api.Authenticate([]byte(cfg.Secret)))

	gophermart := api.NewGophermart(s.logger, svc, []byte(cfg.Secret))
	strictHandler := api.NewStrictHandler(gophermart, nil)
	h := api.HandlerFromMux(strictHandler, r)

	s.srv.Handler = h
}

func ConnectDB(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURI)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	instance, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, err
	}

	m, err := migrate.NewWithDatabaseInstance("file://db/migrations", "postgres", instance)
	if err != nil {
		return nil, err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, err
	}

	return db, nil
}

func Run() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.New()
	if err != nil {
		logger.Info("Configuration error", slog.String("error", err.Error()))
		return
	}

	db, err := ConnectDB(cfg)
	if err != nil {
		logger.Info("DB connection err", slog.String("error", err.Error()))
		return
	}
	defer db.Close()

	client := &http.Client{}
	repo := storage.New(db)
	svc := service.New(repo, client, []byte(cfg.Secret), cfg.AccrualAddress)

	server := NewServer(logger, cfg.Address)
	server.RegisterHandlers(cfg, svc)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go server.HandleShutdown(ctx, &wg)

	logger.Info("Server is listening on", slog.String("address", cfg.Address))
	if err := server.ListenAndServe(cfg); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Info("Starting server error", slog.String("error", err.Error()))
		return
	}

	wg.Wait()
}

func (s *Server) ListenAndServe(cfg *config.Config) error {
	return s.srv.ListenAndServe()
}

func (s *Server) HandleShutdown(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	<-ctx.Done()
	s.logger.Info("Shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.srv.Shutdown(ctx); err != nil {
		s.logger.Info("Shutdown server error", slog.String("error", err.Error()))
		return
	}

	s.logger.Info("Server stopped gracefully")
}
