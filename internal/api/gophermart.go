//go:generate oapi-codegen --config=../../api/types.cfg.yaml ../../api/api.yaml
//go:generate oapi-codegen --config=../../api/server.cfg.yaml ../../api/api.yaml

package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/v-starostin/gophermart/internal/currency"
	"github.com/v-starostin/gophermart/internal/luhn"
	"github.com/v-starostin/gophermart/internal/model"
	"github.com/v-starostin/gophermart/internal/service"
	"github.com/v-starostin/gophermart/internal/storage"
)

type Service interface {
	RegisterUser(login, password string) error
	Authenticate(login, password string) (string, error)
	UploadOrder(userID uuid.UUID, orderNumber string) error
	GetOrders(userID uuid.UUID) ([]model.Order, error)
	WithdrawRequest(userID uuid.UUID, orderNumber string, sum float64) error
	GetBalance(userID uuid.UUID) (float64, float64, error)
	GetWithdrawals(userID uuid.UUID) ([]model.Withdrawal, error)
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
					Message: err.Error(),
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
				Message: err.Error(),
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
	userID, ok := ctx.Value("userID").(uuid.UUID)
	if !ok {
		return GetOrders500JSONResponse{Code: http.StatusInternalServerError, Message: "Failed to retrieve user ID"}, nil
	}

	orders, err := g.service.GetOrders(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GetOrders204Response{}, nil
		}

		return GetOrders500JSONResponse{Code: http.StatusInternalServerError, Message: err.Error()}, nil
	}

	getOrders200Response := make(GetOrders200JSONResponse, len(orders))
	for i, order := range orders {
		accrual := currency.ConvertToPrimary(order.Accrual)
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
	userID, ok := ctx.Value("userID").(uuid.UUID)
	if !ok {
		return UploadOrder500JSONResponse{Code: http.StatusInternalServerError, Message: "Internal server error"}, nil
	}

	if valid := luhn.IsValid(*request.Body); !valid {
		return UploadOrder422JSONResponse{Code: http.StatusUnprocessableEntity, Message: "Bad request"}, nil
	}

	if err := g.service.UploadOrder(userID, *request.Body); err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) {
			if pgErr.Code.Name() == "unique_violation" {
				return UploadOrder409JSONResponse{Code: http.StatusConflict, Message: err.Error()}, nil
			}
		}

		if errors.Is(err, service.ErrOrderAlreadyExists) {
			return UploadOrder200Response{}, nil
		}

		return UploadOrder500JSONResponse{Code: http.StatusInternalServerError, Message: err.Error()}, nil
	}

	return UploadOrder202Response{}, nil
}

func (g *Gophermart) GetBalance(ctx context.Context, request GetBalanceRequestObject) (GetBalanceResponseObject, error) {
	userID, ok := ctx.Value("userID").(uuid.UUID)
	if !ok {
		return GetBalance500JSONResponse{Code: http.StatusInternalServerError, Message: "Internal server error"}, nil
	}

	balance, withdrawn, err := g.service.GetBalance(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GetBalance200JSONResponse{}, nil
		}

		return GetBalance500JSONResponse{Code: http.StatusInternalServerError, Message: err.Error()}, nil
	}

	return GetBalance200JSONResponse{
		Current:   balance,
		Withdrawn: withdrawn,
	}, nil
}

func (g *Gophermart) WithdrawRequest(ctx context.Context, request WithdrawRequestRequestObject) (WithdrawRequestResponseObject, error) {
	userID, ok := ctx.Value("userID").(uuid.UUID)
	if !ok {
		return WithdrawRequest500JSONResponse{Code: http.StatusInternalServerError, Message: "Internal server error"}, nil
	}

	if valid := luhn.IsValid(request.Body.Order); !valid {
		return WithdrawRequest422JSONResponse{Code: http.StatusUnprocessableEntity, Message: "422"}, nil
	}

	err := g.service.WithdrawRequest(userID, request.Body.Order, request.Body.Sum)
	if err != nil {
		if errors.Is(err, storage.ErrInsufficientBalance) {
			return WithdrawRequest402JSONResponse{Code: http.StatusPaymentRequired, Message: storage.ErrInsufficientBalance.Error()}, nil
		}

		return WithdrawRequest500JSONResponse{Code: http.StatusInternalServerError, Message: err.Error()}, nil
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

		return GetWithdrawals500JSONResponse{Code: http.StatusInternalServerError, Message: err.Error()}, nil
	}

	ws := make(GetWithdrawals200JSONResponse, len(withdrawals))
	for i, withdrawal := range withdrawals {
		ws[i] = Withdraw{
			Order:       withdrawal.Order,
			Sum:         currency.ConvertToPrimary(withdrawal.Sum),
			ProcessedAt: &withdrawal.ProcessedAt,
		}
	}

	return ws, nil
}
