package service_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	mmock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/v-starostin/gophermart/internal/mock"
	"github.com/v-starostin/gophermart/internal/model"
	"github.com/v-starostin/gophermart/internal/service"
)

type serviceTestSuite struct {
	suite.Suite
	client  *mock.HTTPClient
	storage *mock.Storage
	service *service.Service
}

func (s *serviceTestSuite) SetupTest() {
	l := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	storage := &mock.Storage{}
	client := &mock.HTTPClient{}
	srv := service.New(l, storage, client, []byte("secret"), "http://example.com")
	s.service = srv
	s.storage = storage
	s.client = client
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(serviceTestSuite))
}

func (s *serviceTestSuite) TestGetWithdrawals() {
	userID, err := uuid.NewRandom()
	s.NoError(err)

	tt := []struct {
		name        string
		withdrawals []model.Withdrawal
		err         error
	}{
		{
			name:        "good case",
			withdrawals: []model.Withdrawal{{Order: "125", Sum: 12.32}, {Order: "109", Sum: 15}},
		},
		{
			name: "bad case",
			err:  errors.New("GetWithdrawals err"),
		},
	}

	for _, test := range tt {
		s.Run(test.name, func() {
			s.storage.On("GetWithdrawals", mmock.Anything, userID).Return(test.withdrawals, test.err).Once()

			got, err := s.service.GetWithdrawals(context.Background(), userID)
			if test.err != nil {
				s.EqualError(err, test.err.Error())
			} else {
				s.NoError(err)
				s.Equal(test.withdrawals, got)
			}
		})
	}
}

func (s *serviceTestSuite) TestGetBalance() {
	userID, err := uuid.NewRandom()
	s.NoError(err)

	tt := []struct {
		name     string
		balance  float64
		withdraw float64
		err      error
	}{
		{
			name:     "good case",
			balance:  12.34,
			withdraw: 15.0,
		},
		{
			name: "bad case",
			err:  errors.New("GetBalance err"),
		},
	}

	for _, test := range tt {
		s.Run(test.name, func() {
			s.storage.On("GetBalance", mmock.Anything, userID).Return(test.balance, test.withdraw, test.err).Once()

			gotBalance, gotWithdraw, err := s.service.GetBalance(context.Background(), userID)
			if test.err != nil {
				s.EqualError(err, test.err.Error())
				s.Equal(0.0, gotBalance)
				s.Equal(0.0, gotWithdraw)
			} else {
				s.NoError(err)
				s.Equal(12.34, gotBalance)
				s.Equal(15.0, gotWithdraw)
			}
		})
	}
}

func (s *serviceTestSuite) TestGetOrders() {
	userID, err := uuid.NewRandom()
	s.NoError(err)

	tt := []struct {
		name   string
		orders []model.Order
		err    error
	}{
		{
			name: "good case",
			orders: []model.Order{
				{Number: "125", Accrual: 12.32, Status: "PROCESSED"},
				{Number: "109", Accrual: 15, Status: "PROCESSED"},
			},
		},
		{
			name: "bad case",
			err:  errors.New("GetOrders err"),
		},
	}

	for _, test := range tt {
		s.Run(test.name, func() {
			s.storage.On("GetOrders", mmock.Anything, userID).Return(test.orders, test.err).Once()

			got, err := s.service.GetOrders(context.Background(), userID)
			if test.err != nil {
				s.EqualError(err, test.err.Error())
			} else {
				s.NoError(err)
				s.Equal(test.orders, got)
			}
		})
	}
}

func (s *serviceTestSuite) TestWithdrawalRequest() {
	userID, err := uuid.NewRandom()
	s.NoError(err)

	tt := []struct {
		name        string
		orderNumber string
		sum         float64
		err         error
	}{
		{
			name:        "good case",
			orderNumber: "125",
			sum:         12.32,
		},
		{
			name: "bad case",
			err:  errors.New("WithdrawalRequest err"),
		},
	}

	for _, test := range tt {
		s.Run(test.name, func() {
			s.storage.On("WithdrawalRequest", mmock.Anything, userID, test.orderNumber, test.sum).Return(test.err).Once()

			err = s.service.WithdrawalRequest(context.Background(), userID, test.orderNumber, test.sum)
			if test.err != nil {
				s.EqualError(err, test.err.Error())
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *serviceTestSuite) TestAuthenticate() {
	login, password := "login", "password"

	tt := []struct {
		name string
		user *model.User
		err  error
	}{
		{
			name: "good case",
			user: &model.User{Login: login, Password: password},
		},
		{
			name: "bad case",
			err:  errors.New("GetUser err"),
		},
	}

	for _, test := range tt {
		s.Run(test.name, func() {
			s.storage.On("GetUser", mmock.Anything, login, password).Return(test.user, test.err).Once()

			_, err := s.service.Authenticate(context.Background(), login, password)
			if test.err != nil {
				s.EqualError(err, test.err.Error())
			} else {
				s.NoError(err)

			}
		})
	}
}

func (s *serviceTestSuite) TestRegisterUser() {
	login, password := "login", "password"

	tt := []struct {
		name string
		user *model.User
		err  error
	}{
		{
			name: "good case",
		},
		{
			name: "bad case",
			err:  errors.New("AddUser err"),
		},
	}

	for _, test := range tt {
		s.Run(test.name, func() {
			s.storage.On("AddUser", mmock.Anything, login, password).Return(test.err).Once()

			err := s.service.RegisterUser(context.Background(), login, password)
			if test.err != nil {
				s.EqualError(err, test.err.Error())
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *serviceTestSuite) TestUploadOrder() {
	orderNumber := "125"
	userID, err := uuid.NewRandom()
	s.NoError(err)

	s.Run("order already exists for current user", func() {
		s.storage.On("GetOrder", mmock.Anything, userID, orderNumber).Once().Return(nil, service.ErrOrderAlreadyExists)
		err = s.service.UploadOrder(context.Background(), userID, orderNumber)
		s.EqualError(err, service.ErrOrderAlreadyExists.Error())
	})

	s.Run("good case", func() {
		order := &model.Order{
			Number:  orderNumber,
			Status:  "PROCESSED",
			Accrual: 12.32,
		}
		respBody, err := json.Marshal(order)
		s.NoError(err)
		res := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}

		s.storage.On("GetOrder", mmock.Anything, userID, orderNumber).Once().Return(nil, sql.ErrNoRows)
		s.client.On("Do", mmock.Anything).Once().Return(res, nil)
		s.storage.On("AddOrder", mmock.Anything, userID, *order).Once().Return(nil)

		err = s.service.UploadOrder(context.Background(), userID, orderNumber)
		s.NoError(err)
	})

	s.Run("error to fetch order, status 500", func() {
		res := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}
		o := model.Order{
			Number: orderNumber,
			Status: "NEW",
		}

		s.storage.On("GetOrder", mmock.Anything, userID, orderNumber).Once().Return(nil, sql.ErrNoRows)
		s.client.On("Do", mmock.Anything).Once().Return(res, nil)
		s.storage.On("AddOrder", mmock.Anything, userID, o).Once().Return(nil)

		err = s.service.UploadOrder(context.Background(), userID, orderNumber)
		s.NoError(err)
	})

	s.Run("error to fetch order info, client error", func() {
		o := model.Order{
			Number: orderNumber,
			Status: "NEW",
		}
		s.storage.On("GetOrder", mmock.Anything, userID, orderNumber).Once().Return(nil, sql.ErrNoRows)
		s.client.On("Do", mmock.Anything).Once().Return(nil, errors.New("client err"))
		s.storage.On("AddOrder", mmock.Anything, userID, o).Once().Return(nil)

		err = s.service.UploadOrder(context.Background(), userID, orderNumber)
		s.NoError(err)
	})
}
