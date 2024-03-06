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
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/v-starostin/gophermart/internal/luhn"
	"github.com/v-starostin/gophermart/internal/model"
)

type Service interface {
	RegisterUser(login, password string) error
	Authenticate(login, password string) (string, error)
	UploadOrder(userID uuid.UUID, number int) error
	GetOrders(userID uuid.UUID) ([]*model.Order, error)
	WithdrawRequest(userID uuid.UUID, order string, sum int) error
	GetBalance(userID uuid.UUID) (int, int, error)
	GetWithdrawals(userID uuid.UUID) ([]*model.Withdrawal, error)
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
	userID, ok := ctx.Value("UserID").(uuid.UUID)
	if !ok {
		return GetOrders500JSONResponse{Code: http.StatusInternalServerError, Message: "Internal server error"}, nil
	}
	orders, err := g.service.GetOrders(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GetOrders204Response{}, nil
		}
		return GetOrders500JSONResponse{Code: http.StatusInternalServerError, Message: "Internal server error"}, nil
	}

	getOrders200Response := make(GetOrders200JSONResponse, len(orders))
	for i, order := range orders {
		accrual := strconv.Itoa(order.Accrual)
		getOrders200Response[i] = Order{
			Number:     &order.Number,
			Status:     &order.Status,
			Accrual:    &accrual,
			UploadedAt: &order.UploadedAt,
		}
	}

	return getOrders200Response, nil
}

func (g *Gophermart) UploadOrder(ctx context.Context, request UploadOrderRequestObject) (UploadOrderResponseObject, error) {
	userID, ok := ctx.Value("UserID").(uuid.UUID)
	if !ok {
		return UploadOrder500JSONResponse{Code: http.StatusBadRequest, Message: "Internal server error"}, nil
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

func (g *Gophermart) GetBalance(ctx context.Context, request GetBalanceRequestObject) (GetBalanceResponseObject, error) {
	userID, ok := ctx.Value("userID").(uuid.UUID)
	if !ok {
		return GetBalance500JSONResponse{Code: http.StatusBadRequest, Message: "Internal server error"}, nil
	}
	balance, withdrawn, err := g.service.GetBalance(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GetBalance200JSONResponse{}, nil
		}
		return GetBalance500JSONResponse{Code: http.StatusInternalServerError, Message: "Internal server error"}, nil
	}
	return GetBalance200JSONResponse{
		Current:   &balance,
		Withdrawn: &withdrawn,
	}, nil
}

func (g *Gophermart) WithdrawRequest(ctx context.Context, request WithdrawRequestRequestObject) (WithdrawRequestResponseObject, error) {
	orderNumber, err := strconv.Atoi(*request.Body.Order)
	if err != nil {
		return WithdrawRequest422JSONResponse{Code: http.StatusBadRequest, Message: "422"}, nil
	}
	if valid := luhn.IsValid(orderNumber); !valid {
		return WithdrawRequest422JSONResponse{Code: http.StatusUnprocessableEntity, Message: "422"}, nil
	}
	userID, ok := ctx.Value("userID").(uuid.UUID)
	if !ok {
		return WithdrawRequest500JSONResponse{Code: http.StatusInternalServerError, Message: "Internal server error"}, nil
	}
	err = g.service.WithdrawRequest(userID, *request.Body.Order, *request.Body.Sum)
	if err != nil {
		return WithdrawRequest500JSONResponse{Code: http.StatusInternalServerError, Message: "Internal server error"}, nil
	}
	return WithdrawRequest200Response{}, nil
}

func (g *Gophermart) GetWithdrawals(ctx context.Context, request GetWithdrawalsRequestObject) (GetWithdrawalsResponseObject, error) {
	userID, ok := ctx.Value("userID").(uuid.UUID)
	if !ok {
		return GetWithdrawals500JSONResponse{Code: http.StatusBadRequest, Message: "Internal server error"}, nil
	}
	withdrawals, err := g.service.GetWithdrawals(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GetWithdrawals204Response{}, nil
		}
		return GetWithdrawals500JSONResponse{Code: http.StatusInternalServerError, Message: "Internal server error"}, nil
	}
	ws := make(GetWithdrawals200JSONResponse, len(withdrawals))
	for i, withdrawal := range withdrawals {
		ws[i] = struct {
			Order       *string    `json:"order,omitempty"`
			ProcessedAt *time.Time `json:"processed_at,omitempty"`
			Sum         *int       `json:"sum,omitempty"`
		}{
			Order:       &withdrawal.Order,
			Sum:         &withdrawal.Sum,
			ProcessedAt: &withdrawal.ProcessedAt,
		}
	}

	return ws, nil
}
