package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

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
	cfg, err := config.New()
	if err != nil {
		log.Println("configuration err", err)
		return
	}

	db, err := sql.Open("postgres", cfg.DatabaseURI)
	if err != nil {
		log.Println("database connection err", err)
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Println("db ping err", err)
		return
	}

	instance, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Println(err)
		return
	}

	m, err := migrate.NewWithDatabaseInstance("file://db/migrations", "postgres", instance)
	if err != nil {
		log.Println(err)
		return
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Println(err)
		return
	}

	swagger, err := api.GetSwagger()
	if err != nil {
		log.Println(err)
		return
	}

	swagger.Servers = nil

	repo := storage.New(db)
	srv := service.New(repo, []byte(cfg.Secret), cfg.AccrualAddress)
	gophermart := api.NewGophermart(srv, []byte(cfg.Secret))
	strictHandler := api.NewStrictHandler(gophermart, nil)

	r := chi.NewRouter()
	r.Use(middleware.OapiRequestValidator(swagger))
	r.Use(api.Authenticate([]byte(cfg.Secret)))

	h := api.HandlerFromMux(strictHandler, r)

	server := &http.Server{
		Addr:    cfg.Address,
		Handler: h,
	}

	log.Println("Server is listening on", cfg.Address)
	if err := server.ListenAndServe(); err != nil || !errors.Is(err, http.ErrServerClosed) {
		log.Println(err)
		return
	}
}
