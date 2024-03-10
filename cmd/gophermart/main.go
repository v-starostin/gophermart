package main

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"

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

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.New()
	if err != nil {
		logger.Info("Configuration error", slog.String("error", err.Error()))
		return
	}

	db, err := sql.Open("postgres", cfg.DatabaseURI)
	if err != nil {
		logger.Info("DB connection error", slog.String("error", err.Error()))
		return
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		logger.Info("DB ping error", slog.String("error", err.Error()))
		return
	}

	instance, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		logger.Info("DB migration 'WithInstance' error", slog.String("error", err.Error()))
		return
	}

	m, err := migrate.NewWithDatabaseInstance("file://db/migrations", "postgres", instance)
	if err != nil {
		logger.Info("DB migration 'NewWithDatabaseInstance' error", slog.String("error", err.Error()))
		return
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		logger.Info("DB migration 'Up' error", slog.String("error", err.Error()))
		return
	}

	swagger, err := api.GetSwagger()
	if err != nil {
		logger.Info("'GetSwagger' error", slog.String("error", err.Error()))
		return
	}

	swagger.Servers = nil

	repo := storage.New(db)
	srv := service.New(repo, []byte(cfg.Secret), cfg.AccrualAddress)
	gophermart := api.NewGophermart(logger, srv, []byte(cfg.Secret))
	strictHandler := api.NewStrictHandler(gophermart, nil)

	r := chi.NewRouter()
	r.Use(middleware.OapiRequestValidator(swagger))
	r.Use(api.Authenticate([]byte(cfg.Secret)))

	h := api.HandlerFromMux(strictHandler, r)

	server := &http.Server{
		Addr:    cfg.Address,
		Handler: h,
	}

	logger.Info("Server is listening on", cfg.Address)
	if err := server.ListenAndServe(); err != nil || !errors.Is(err, http.ErrServerClosed) {
		logger.Info("Starting server error", slog.String("error", err.Error()))
		return
	}
}
