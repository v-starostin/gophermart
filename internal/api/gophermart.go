//go:generate oapi-codegen --config=../../api/types.cfg.yaml ../../api/api.yaml
//go:generate oapi-codegen --config=../../api/server.cfg.yaml ../../api/api.yaml

package api

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/lib/pq"

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
	WithdrawalRequest(userID uuid.UUID, orderNumber string, sum float64) error
	GetBalance(userID uuid.UUID) (float64, float64, error)
	GetWithdrawals(userID uuid.UUID) ([]model.Withdrawal, error)
}

type Gophermart struct {
	logger  *slog.Logger
	service Service
	secret  []byte
}

var _ StrictServerInterface = (*Gophermart)(nil)

func NewGophermart(logger *slog.Logger, service Service, secret []byte) *Gophermart {
	return &Gophermart{
		logger:  logger,
		service: service,
		secret:  secret,
	}
}

func (g *Gophermart) RegisterUser(ctx context.Context, request RegisterUserRequestObject) (RegisterUserResponseObject, error) {
	err := g.service.RegisterUser(request.Body.Login, request.Body.Password)
	if err != nil {
		g.logger.Info("Register user error", slog.String("error", err.Error()))
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code.Name() == "unique_violation" {
				return RegisterUser409JSONResponse{
					Message: "User already exists",
				}, nil
			}
		}

		return RegisterUser500JSONResponse{
			Message: "Internal server error",
		}, nil
	}

	token, err := g.service.Authenticate(request.Body.Login, request.Body.Password)
	if err != nil {
		g.logger.Info("Authentication error", slog.String("error", err.Error()))
		return RegisterUser500JSONResponse{
			Message: "Internal server error",
		}, nil
	}

	return RegisterUser200Response{
		RegisterUser200ResponseHeaders{
			Authorization: "Bearer " + token,
		},
	}, nil
}

func (g *Gophermart) LoginUser(ctx context.Context, request LoginUserRequestObject) (LoginUserResponseObject, error) {
	token, err := g.service.Authenticate(request.Body.Login, request.Body.Password)
	if err != nil {
		g.logger.Info("Authentication error", slog.String("error", err.Error()))
		if errors.Is(err, sql.ErrNoRows) {
			return LoginUser401JSONResponse{
				Message: "Unauthorized",
			}, nil
		}

		return LoginUser500JSONResponse{
			Message: "Internal server error",
		}, nil
	}

	return LoginUser200Response{
		LoginUser200ResponseHeaders{
			Authorization: "Bearer " + token,
		},
	}, nil
}

func (g *Gophermart) GetOrders(ctx context.Context, request GetOrdersRequestObject) (GetOrdersResponseObject, error) {
	userID := ctx.Value(KeyUserID).(uuid.UUID)

	orders, err := g.service.GetOrders(userID)
	if err != nil {
		g.logger.Info("Get orders error", slog.String("error", err.Error()))
		if errors.Is(err, sql.ErrNoRows) {
			return GetOrders204Response{}, nil
		}

		return GetOrders500JSONResponse{
			Message: "Internal server error",
		}, nil
	}

	getOrders200Response := make(GetOrders200JSONResponse, len(orders))
	for i, order := range orders {
		accrual := order.Accrual
		getOrders200Response[i] = Order{
			Number:     order.Number,
			Status:     order.Status,
			Accrual:    &accrual,
			UploadedAt: order.UploadedAt,
		}
	}

	return getOrders200Response, nil
}

func (g *Gophermart) UploadOrder(ctx context.Context, request UploadOrderRequestObject) (UploadOrderResponseObject, error) {
	userID := ctx.Value(KeyUserID).(uuid.UUID)

	if valid := luhn.IsValid(*request.Body); !valid {
		return UploadOrder422JSONResponse{
			Message: "Invalid order number",
		}, nil
	}

	if err := g.service.UploadOrder(userID, *request.Body); err != nil {
		g.logger.Info("Upload order error", slog.String("error", err.Error()))
		var pgErr *pq.Error
		if errors.As(err, &pgErr) {
			if pgErr.Code.Name() == "unique_violation" {
				return UploadOrder409JSONResponse{
					Message: "Order already exists",
				}, nil
			}
		}

		if errors.Is(err, service.ErrOrderAlreadyExists) {
			return UploadOrder200Response{}, nil
		}

		return UploadOrder500JSONResponse{
			Message: "Internal server error",
		}, nil
	}

	return UploadOrder202Response{}, nil
}

func (g *Gophermart) GetBalance(ctx context.Context, request GetBalanceRequestObject) (GetBalanceResponseObject, error) {
	userID := ctx.Value(KeyUserID).(uuid.UUID)

	balance, withdrawn, err := g.service.GetBalance(userID)
	if err != nil {
		g.logger.Info("Get balance error", slog.String("error", err.Error()))
		if errors.Is(err, sql.ErrNoRows) {
			return GetBalance200JSONResponse{}, nil
		}

		return GetBalance500JSONResponse{
			Message: "Internal server error",
		}, nil
	}

	return GetBalance200JSONResponse{
		Current:   balance,
		Withdrawn: withdrawn,
	}, nil
}

func (g *Gophermart) WithdrawalRequest(ctx context.Context, request WithdrawalRequestRequestObject) (WithdrawalRequestResponseObject, error) {
	userID := ctx.Value(KeyUserID).(uuid.UUID)

	if valid := luhn.IsValid(request.Body.Order); !valid {
		return WithdrawalRequest422JSONResponse{
			Message: "Invalid order number",
		}, nil
	}

	err := g.service.WithdrawalRequest(userID, request.Body.Order, request.Body.Sum)
	if err != nil {
		g.logger.Info("Withdrawal request error", slog.String("error", err.Error()))
		if errors.Is(err, storage.ErrInsufficientBalance) {
			return WithdrawalRequest402JSONResponse{
				Message: storage.ErrInsufficientBalance.Error(),
			}, nil
		}

		return WithdrawalRequest500JSONResponse{
			Message: "Internal server error",
		}, nil
	}

	return WithdrawalRequest200Response{}, nil
}

func (g *Gophermart) GetWithdrawals(ctx context.Context, request GetWithdrawalsRequestObject) (GetWithdrawalsResponseObject, error) {
	userID := ctx.Value(KeyUserID).(uuid.UUID)

	withdrawals, err := g.service.GetWithdrawals(userID)
	if err != nil {
		g.logger.Info("Get withdrawals error", slog.String("error", err.Error()))
		if errors.Is(err, sql.ErrNoRows) {
			return GetWithdrawals204Response{}, nil
		}

		return GetWithdrawals500JSONResponse{
			Message: "Internal server error",
		}, nil
	}

	ws := make(GetWithdrawals200JSONResponse, len(withdrawals))
	for i, withdrawal := range withdrawals {
		processedAt := withdrawal.ProcessedAt
		ws[i] = Withdrawal{
			Order:       withdrawal.Order,
			Sum:         withdrawal.Sum,
			ProcessedAt: &processedAt,
		}
	}

	return ws, nil
}
