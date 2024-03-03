//go:generate oapi-codegen --config=../../api/types.cfg.yaml ../../api/api.yaml
//go:generate oapi-codegen --config=../../api/server.cfg.yaml ../../api/api.yaml

package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/lib/pq"
)

type Service interface {
	RegisterUser(login, password string) error
	Authenticate(login, password string) (string, error)
}

type Gophermart struct {
	service Service
}

var _ StrictServerInterface = (*Gophermart)(nil)

func NewGophermart(s Service) *Gophermart {
	return &Gophermart{service: s}
}

func (g *Gophermart) RegisterUser(ctx context.Context, request RegisterUserRequestObject) (RegisterUserResponseObject, error) {
	err := g.service.RegisterUser(request.Body.Login, request.Body.Password)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code.Name() == "23505" {
				return RegisterUser409JSONResponse{
					Code:    http.StatusConflict,
					Message: "User already exists",
				}, nil
			}
		}

		return RegisterUser500JSONResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		}, nil
	}

	token, err := g.service.Authenticate(request.Body.Login, request.Body.Password)
	if err != nil {
		return RegisterUser500JSONResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		}, nil
	}

	return RegisterUser200Response{RegisterUser200ResponseHeaders{Authorization: token}}, nil
}

func (g *Gophermart) LoginUser(ctx context.Context, request LoginUserRequestObject) (LoginUserResponseObject, error) {
	token, err := g.service.Authenticate(request.Body.Login, request.Body.Password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginUser401JSONResponse{
				Code:    http.StatusUnauthorized,
				Message: "User do not exist",
			}, nil
		}

		return LoginUser500JSONResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		}, nil
	}

	return LoginUser200Response{LoginUser200ResponseHeaders{Authorization: token}}, nil
}

func (g *Gophermart) GetOrders(ctx context.Context, request GetOrdersRequestObject) (GetOrdersResponseObject, error) {
	return nil, nil
}

func (g *Gophermart) UploadOrder(ctx context.Context, request UploadOrderRequestObject) (UploadOrderResponseObject, error) {
	return nil, nil
}

func (g *Gophermart) Ping(ctx context.Context, request PingRequestObject) (PingResponseObject, error) {
	return Ping200JSONResponse("pong"), nil
}
