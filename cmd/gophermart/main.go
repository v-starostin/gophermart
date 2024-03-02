package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	middleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/v-starostin/gophermart/internal/api"
	"github.com/v-starostin/gophermart/internal/handler"
)

func main() {
	swagger, err := api.GetSwagger()
	if err != nil {
		log.Println(err)
		return
	}

	swagger.Servers = nil

	gophermart := handler.NewGophermart()
	strictHandler := api.NewStrictHandler(gophermart, nil)

	r := chi.NewRouter()
	r.Use(middleware.OapiRequestValidator(swagger))

	h := api.HandlerFromMux(strictHandler, r)

	server := &http.Server{
		Addr:    ":8080",
		Handler: h,
	}

	if err := server.ListenAndServe(); err != nil || !errors.Is(err, http.ErrServerClosed) {
		log.Println(err)
		return
	}
}
