package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lib/pq"
	middleware "github.com/oapi-codegen/nethttp-middleware"
	mmock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/v-starostin/gophermart/internal/api"
	"github.com/v-starostin/gophermart/internal/mock"
	"github.com/v-starostin/gophermart/internal/model"
	"github.com/v-starostin/gophermart/internal/service"
	"github.com/v-starostin/gophermart/internal/storage"
)

type apiTestSuite struct {
	suite.Suite
	r       *chi.Mux
	service *mock.Service
}

func (suite *apiTestSuite) SetupTest() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := &mock.Service{}
	swagger, err := api.GetSwagger()
	suite.NoError(err)
	swagger.Servers = nil
	r := chi.NewRouter()
	r.Use(middleware.OapiRequestValidator(swagger))
	g := api.NewGophermart(logger, srv, []byte("secret"))
	sh := api.NewStrictHandler(g, nil)
	api.HandlerFromMux(sh, r)
	suite.r = r
	suite.service = srv
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(apiTestSuite))
}

func (suite *apiTestSuite) TestRegisterUser() {
	user := api.RegisterUserJSONRequestBody{
		Login:    "login",
		Password: "password",
	}

	b, err := json.Marshal(user)
	suite.NoError(err)

	suite.Run("Register user", func() {
		req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/user/register", bytes.NewReader(b))
		suite.NoError(err)
		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.service.On("RegisterUser", "login", "password").Once().Return(nil)
		suite.service.On("Authenticate", "login", "password").Once().Return("token", nil)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()

		suite.Equal("Bearer token", res.Header.Get("Authorization"))
	})

	suite.Run("User already exists", func() {
		req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/user/register", bytes.NewReader(b))
		suite.NoError(err)
		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		pqErr := &pq.Error{
			Code: pq.ErrorCode("23505"),
		}
		suite.service.On("RegisterUser", "login", "password").Once().Return(pqErr)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.RegisterUser409JSONResponse{
			Code:    409,
			Message: "User already exists",
		}

		var got api.RegisterUser409JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("Internal server error (Register user)", func() {
		req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/user/register", bytes.NewReader(b))
		suite.NoError(err)
		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.service.On("RegisterUser", "login", "password").Once().Return(errors.New("RegisterUser err"))
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.RegisterUser500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}

		var got api.RegisterUser500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("Internal server error (Authenticate user)", func() {
		req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/user/register", bytes.NewReader(b))
		suite.NoError(err)
		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.service.On("RegisterUser", "login", "password").Once().Return(nil)
		suite.service.On("Authenticate", "login", "password").Once().Return("", errors.New("Authenticate err"))
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.RegisterUser500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}

		var got api.RegisterUser500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

}

func (suite *apiTestSuite) TestLoginUser() {
	user := api.LoginUserJSONRequestBody{
		Login:    "login",
		Password: "password",
	}

	b, err := json.Marshal(user)
	suite.NoError(err)

	suite.Run("Login user", func() {
		req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/user/login", bytes.NewReader(b))
		suite.NoError(err)
		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.service.On("Authenticate", "login", "password").Once().Return("token", nil)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()

		suite.Equal("Bearer token", res.Header.Get("Authorization"))
	})

	suite.Run("User unauthorized", func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:8080/api/user/login", bytes.NewReader(b))
		suite.NoError(err)
		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.service.On("Authenticate", "login", "password").Once().Return("", sql.ErrNoRows)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.LoginUser401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}

		var got api.LoginUser401JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("Internal server error", func() {
		req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/user/login", bytes.NewReader(b))
		suite.NoError(err)
		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.service.On("Authenticate", "login", "password").Once().Return("", errors.New("Authenticate err"))
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.LoginUser500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}

		var got api.LoginUser500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

}

func (suite *apiTestSuite) TestGetBalance() {
	userID, err := uuid.NewRandom()
	suite.NoError(err)

	suite.Run("Get balance", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8080/api/user/balance", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		suite.service.On("GetBalance", userID).Once().Return(12.32, 5.04, nil)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.GetBalance200JSONResponse{
			Current:   12.32,
			Withdrawn: 5.04,
		}

		var got api.GetBalance200JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("No user ID", func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8080/api/user/balance", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.GetBalance500JSONResponse{
			Code:    500,
			Message: "Can not retrieve user ID",
		}

		var got api.GetBalance500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("No balance for current user", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8080/api/user/balance", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		suite.service.On("GetBalance", userID).Once().Return(0.0, 0.0, sql.ErrNoRows)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.GetBalance200JSONResponse{
			Current:   0.0,
			Withdrawn: 0.0,
		}

		var got api.GetBalance200JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("Internal server error", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8080/api/user/balance", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		suite.service.On("GetBalance", userID).Once().Return(0.0, 0.0, errors.New("GetBalance err"))
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.GetBalance500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}

		var got api.GetBalance500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})
}

func (suite *apiTestSuite) TestGetWithdrawals() {
	userID, err := uuid.NewRandom()
	suite.NoError(err)

	suite.Run("Get withdrawals", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8080/api/user/withdrawals", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		// todo: fix time
		withdrawals := []model.Withdrawal{
			{Order: "125", Sum: 255.54, ProcessedAt: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
			{Order: "2006", Sum: 1024.0, ProcessedAt: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
		}

		suite.service.On("GetWithdrawals", userID).Once().Return(withdrawals, nil)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := make(api.GetWithdrawals200JSONResponse, len(withdrawals))
		for i, withdrawal := range withdrawals {
			processedAt := withdrawal.ProcessedAt
			expected[i] = api.Withdrawal{
				Order:       withdrawal.Order,
				Sum:         withdrawal.Sum,
				ProcessedAt: &processedAt,
			}
		}

		var got api.GetWithdrawals200JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("No user ID", func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8080/api/user/withdrawals", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.GetWithdrawals500JSONResponse{
			Code:    500,
			Message: "Can not retrieve user ID",
		}

		var got api.GetWithdrawals500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("No balance for current user", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8080/api/user/withdrawals", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		suite.service.On("GetWithdrawals", userID).Once().Return(nil, sql.ErrNoRows)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()

		suite.Equal(http.StatusNoContent, res.StatusCode)
	})

	suite.Run("Internal server error", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8080/api/user/withdrawals", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		suite.service.On("GetWithdrawals", userID).Once().Return(nil, errors.New("GetWithdrawals err"))
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.GetWithdrawals500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}

		var got api.GetWithdrawals500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})
}

func (suite *apiTestSuite) TestGetOrders() {
	userID, err := uuid.NewRandom()
	suite.NoError(err)

	suite.Run("Get orders", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8080/api/user/orders", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		// todo: fix time
		orders := []model.Order{
			{Number: "125", Accrual: 255.54, Status: "PROCESSED", UploadedAt: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
			{Number: "2006", Accrual: 1024.0, Status: "PROCESSED", UploadedAt: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)},
		}

		suite.service.On("GetOrders", userID).Once().Return(orders, nil)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := make(api.GetOrders200JSONResponse, len(orders))
		for i, order := range orders {
			accrual := order.Accrual
			expected[i] = api.Order{
				Number:     order.Number,
				Status:     order.Status,
				Accrual:    &accrual,
				UploadedAt: order.UploadedAt,
			}
		}

		var got api.GetOrders200JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("No user ID", func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8080/api/user/orders", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.GetOrders500JSONResponse{
			Code:    500,
			Message: "Can not retrieve user ID",
		}

		var got api.GetOrders500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("No orders for current user", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8080/api/user/orders", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		suite.service.On("GetOrders", userID).Once().Return(nil, sql.ErrNoRows)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()

		suite.Equal(http.StatusNoContent, res.StatusCode)
	})

	suite.Run("Internal server error", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8080/api/user/orders", nil)
		suite.NoError(err)

		rr := httptest.NewRecorder()

		suite.service.On("GetOrders", userID).Once().Return(nil, errors.New("GetOrders err"))
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.GetOrders500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}

		var got api.GetOrders500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})
}

func (suite *apiTestSuite) TestUploadOrder() {
	userID, err := uuid.NewRandom()
	suite.NoError(err)

	suite.Run("Upload order", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/user/orders", strings.NewReader("125"))
		suite.NoError(err)

		req.Header.Add("Content-Type", "text/plain")

		rr := httptest.NewRecorder()

		suite.service.On("UploadOrder", userID, mmock.Anything).Once().Return(nil)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()

		suite.Equal(http.StatusAccepted, res.StatusCode)
	})

	suite.Run("No user ID", func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:8080/api/user/orders", strings.NewReader("125"))
		suite.NoError(err)

		req.Header.Add("Content-Type", "text/plain")

		rr := httptest.NewRecorder()

		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.UploadOrder500JSONResponse{
			Code:    500,
			Message: "Can not retrieve user ID",
		}

		var got api.UploadOrder500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("Invalid order number", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/user/orders", strings.NewReader("126"))
		suite.NoError(err)

		req.Header.Add("Content-Type", "text/plain")

		rr := httptest.NewRecorder()

		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.UploadOrder422JSONResponse{
			Code:    422,
			Message: "Invalid order number",
		}

		var got api.UploadOrder422JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("Order already exists", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/user/orders", strings.NewReader("125"))
		suite.NoError(err)

		req.Header.Add("Content-Type", "text/plain")

		rr := httptest.NewRecorder()

		pqErr := &pq.Error{
			Code: "23505",
		}

		suite.service.On("UploadOrder", userID, mmock.Anything).Once().Return(pqErr)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.UploadOrder409JSONResponse{
			Code:    409,
			Message: "Order already exists",
		}

		var got api.UploadOrder409JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("Order for current user already exists", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/user/orders", strings.NewReader("125"))
		suite.NoError(err)

		req.Header.Add("Content-Type", "text/plain")

		rr := httptest.NewRecorder()

		suite.service.On("UploadOrder", userID, mmock.Anything).Once().Return(service.ErrOrderAlreadyExists)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()

		suite.Equal(http.StatusOK, res.StatusCode)
	})

	suite.Run("Internal server error", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/user/orders", strings.NewReader("125"))
		suite.NoError(err)

		req.Header.Add("Content-Type", "text/plain")

		rr := httptest.NewRecorder()

		suite.service.On("UploadOrder", userID, mmock.Anything).Once().Return(errors.New("UploadOrder err"))
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.UploadOrder500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}

		var got api.UploadOrder500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})
}

func (suite *apiTestSuite) TestWithdrawalRequest() {
	userID, err := uuid.NewRandom()
	suite.NoError(err)

	processedAt := time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
	withdrawal := api.WithdrawalRequestJSONRequestBody{
		Order:       "125",
		Sum:         125.78,
		ProcessedAt: &processedAt,
	}

	b, err := json.Marshal(withdrawal)
	suite.NoError(err)

	suite.Run("Withdrawal request", func() {

		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/user/balance/withdraw", bytes.NewReader(b))
		suite.NoError(err)

		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.service.On("WithdrawalRequest", userID, withdrawal.Order, withdrawal.Sum).Once().Return(nil)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()

		suite.Equal(http.StatusOK, res.StatusCode)
	})

	suite.Run("No user ID", func() {

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:8080/api/user/balance/withdraw", bytes.NewReader(b))
		suite.NoError(err)

		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.WithdrawalRequest500JSONResponse{
			Code:    500,
			Message: "Can not retrieve user ID",
		}

		var got api.WithdrawalRequest500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("Invalid order number", func() {
		withdrawal := api.WithdrawalRequestJSONRequestBody{
			Order:       "126",
			Sum:         125.78,
			ProcessedAt: &processedAt,
		}

		b, err := json.Marshal(withdrawal)
		suite.NoError(err)

		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/user/balance/withdraw", bytes.NewReader(b))
		suite.NoError(err)

		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.WithdrawalRequest422JSONResponse{
			Code:    422,
			Message: "Invalid order number",
		}

		var got api.WithdrawalRequest422JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("Insufficient balance", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/user/balance/withdraw", bytes.NewReader(b))
		suite.NoError(err)

		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.service.On("WithdrawalRequest", userID, withdrawal.Order, withdrawal.Sum).Once().Return(storage.ErrInsufficientBalance)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.WithdrawalRequest402JSONResponse{
			Code:    402,
			Message: storage.ErrInsufficientBalance.Error(),
		}

		var got api.WithdrawalRequest402JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})

	suite.Run("Internal server error", func() {
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/user/balance/withdraw", bytes.NewReader(b))
		suite.NoError(err)

		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.service.On("WithdrawalRequest", userID, withdrawal.Order, withdrawal.Sum).Once().Return(errors.New("WithdrawalRequest err"))
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)

		expected := api.WithdrawalRequest500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}

		var got api.WithdrawalRequest500JSONResponse
		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		suite.Equal(expected, got)
	})
}
