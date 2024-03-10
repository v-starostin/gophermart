package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lib/pq"
	middleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/stretchr/testify/suite"
	"github.com/v-starostin/gophermart/internal/api"
	"github.com/v-starostin/gophermart/internal/mock"
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
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:8080/api/user/register", bytes.NewReader(b))
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
	suite.Run("Login user", func() {
		user := api.User{
			Login:    "login",
			Password: "password",
		}

		b, err := json.Marshal(user)
		suite.NoError(err)
		userID, _ := uuid.NewRandom()
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/user/register", bytes.NewReader(b))
		suite.NoError(err)

		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.service.On("RegisterUser", "login", "password").Once().Return(nil)
		suite.service.On("Authenticate", "login", "password").Once().Return("token", nil)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		//defer res.Body.Close()
		//resBody, err := io.ReadAll(res.Body)
		//suite.NoError(err)
		log.Println("response header", string(res.Header.Get("Authorization")))

		suite.Equal("Bearer token", res.Header.Get("Authorization"))

	})
}

func (suite *apiTestSuite) TestGetBalance() {
	suite.Run("Good case", func() {

		//b, err := json.Marshal(user)
		//suite.NoError(err)
		userID, _ := uuid.NewRandom()
		ctx := context.WithValue(context.Background(), api.KeyUserID, userID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8080/api/user/balance", nil)
		suite.NoError(err)

		//req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		suite.service.On("GetBalance", userID).Once().Return(12.32, 5.04, nil)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)
		log.Println("response body", string(resBody))

		expected := api.GetBalance200JSONResponse{
			Current:   12.32,
			Withdrawn: 5.04,
		}

		var got api.GetBalance200JSONResponse

		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		log.Println("balance", got)

		suite.Equal(expected, got)

	})

	suite.Run("No user ID", func() {

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8080/api/user/balance", nil)
		suite.NoError(err)

		//req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		//suite.service.On("GetBalance", userID).Once().Return(12.32, 5.04, nil)
		suite.r.ServeHTTP(rr, req)
		res := rr.Result()
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		suite.NoError(err)
		log.Println("response body", string(resBody))

		expected := api.GetBalance500JSONResponse{
			Code:    500,
			Message: "Can not retrieve user ID",
		}

		var got api.GetBalance500JSONResponse

		err = json.Unmarshal(resBody, &got)
		suite.NoError(err)

		log.Println("balance", got)

		suite.Equal(expected, got)

	})
}
