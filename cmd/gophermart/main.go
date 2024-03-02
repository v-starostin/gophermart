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
	_ "github.com/jackc/pgx/v5/stdlib"
	middleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/v-starostin/gophermart/internal/api"
	"github.com/v-starostin/gophermart/internal/config"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Println(err)
		return
	}

	db, err := sql.Open("pgx", cfg.DatabaseURI)
	if err != nil {
		log.Println(err)
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Println(err)
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

	var service api.Service
	gophermart := api.NewGophermart(service)
	strictHandler := api.NewStrictHandler(gophermart, nil)

	r := chi.NewRouter()
	r.Use(middleware.OapiRequestValidator(swagger))

	h := api.HandlerFromMux(strictHandler, r)

	server := &http.Server{
		Addr:    cfg.Address,
		Handler: h,
	}

	if err := server.ListenAndServe(); err != nil || !errors.Is(err, http.ErrServerClosed) {
		log.Println(err)
		return
	}
}
