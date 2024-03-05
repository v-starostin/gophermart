//go:generate oapi-codegen --config=../../api/types.cfg.yaml ../../api/api.yaml
//go:generate oapi-codegen --config=../../api/server.cfg.yaml ../../api/api.yaml

package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/v-starostin/gophermart/internal/luhn"
)

type Service interface {
	RegisterUser(login, password string) error
	Authenticate(login, password string) (string, error)
	UploadOrder(userID uuid.UUID, number int) error
}

type Gophermart struct {
	service Service
	secret  []byte
}

var _ StrictServerInterface = (*Gophermart)(nil)

func NewGophermart(service Service, secret []byte) *Gophermart {
	return &Gophermart{service, secret}
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
	userID, ok := ctx.Value("UserID").(uuid.UUID)
	if !ok {
		return UploadOrder400JSONResponse{Code: http.StatusBadRequest, Message: "Bad request"}, nil
	}
	number, err := strconv.Atoi(*request.Body)
	if err != nil {
		return UploadOrder400JSONResponse{Code: http.StatusBadRequest, Message: "Bad request"}, nil
	}
	if valid := luhn.IsValid(number); !valid {
		return UploadOrder422JSONResponse{Code: http.StatusUnprocessableEntity, Message: "Bad request"}, nil
	}
	if err := g.service.UploadOrder(userID, number); err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) {
			if pgErr.Code.Name() == "23505" {
				return UploadOrder409JSONResponse{Code: http.StatusConflict, Message: "Bad request"}, nil
			}
		}
		if errors.Is(err, fmt.Errorf("order %d already exists for user %s", number, userID)) {
			return UploadOrder200Response{}, nil
		}
		return UploadOrder500JSONResponse{Code: http.StatusInternalServerError, Message: "Internal server error"}, nil
	}

	return UploadOrder202Response{}, nil
}
