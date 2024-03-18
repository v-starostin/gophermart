package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/v-starostin/gophermart/internal/model"
)

const (
	accrualAPIFormat = "%s/api/orders/%s"
)

var (
	ErrOrderAlreadyExists = errors.New("order already exists for current user")
	ErrHittingRateLimit   = errors.New("hitting rate limit")
)

type Storage interface {
	AddUser(ctx context.Context, login, password string) error
	GetUser(ctx context.Context, login, password string) (*model.User, error)
	AddOrder(ctx context.Context, userID uuid.UUID, order model.Order) error
	UpdateOrder(ctx context.Context, userID uuid.UUID, order model.Order) error
	GetOrder(ctx context.Context, userID uuid.UUID, orderNumber string) (*model.Order, error)
	GetOrders(ctx context.Context, userID uuid.UUID) ([]model.Order, error)
	WithdrawalRequest(ctx context.Context, userID uuid.UUID, orderNumber string, sum float64) error
	GetBalance(ctx context.Context, userID uuid.UUID) (float64, float64, error)
	GetWithdrawals(ctx context.Context, userID uuid.UUID) ([]model.Withdrawal, error)
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Service struct {
	logger     *slog.Logger
	storage    Storage
	client     HTTPClient
	secret     []byte
	accrualURL string
	timeout    atomic.Int64
}

func New(logger *slog.Logger, storage Storage, client HTTPClient, secret []byte, url string) *Service {
	return &Service{
		logger:     logger,
		storage:    storage,
		secret:     secret,
		accrualURL: url,
		client:     client,
	}
}

func (s *Service) GetWithdrawals(ctx context.Context, userID uuid.UUID) ([]model.Withdrawal, error) {
	return s.storage.GetWithdrawals(ctx, userID)
}

func (s *Service) GetBalance(ctx context.Context, userID uuid.UUID) (float64, float64, error) {
	return s.storage.GetBalance(ctx, userID)
}

func (s *Service) GetOrders(ctx context.Context, userID uuid.UUID) ([]model.Order, error) {
	return s.storage.GetOrders(ctx, userID)
}

func (s *Service) WithdrawalRequest(ctx context.Context, userID uuid.UUID, orderNumber string, sum float64) error {
	return s.storage.WithdrawalRequest(ctx, userID, orderNumber, sum)
}

func (s *Service) UploadOrder(ctx context.Context, userID uuid.UUID, orderNumber string) error {
	_, err := s.storage.GetOrder(ctx, userID, orderNumber)
	if err == nil {
		return ErrOrderAlreadyExists
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var o *model.Order
	o, err = s.fetchOrder(ctx, orderNumber)
	if err != nil {
		// if 3rd party service is not available, log error and create new order with NEW status
		s.logger.Info("Failed to fetch order status, creating NEW order", slog.String("error", err.Error()))
		o = &model.Order{
			Number: orderNumber,
			Status: "NEW",
		}
	}

	return s.storage.AddOrder(ctx, userID, *o)
}

func (s *Service) RegisterUser(ctx context.Context, login, password string) error {
	return s.storage.AddUser(ctx, login, password)
}

func (s *Service) Authenticate(ctx context.Context, login, password string) (string, error) {
	user, err := s.storage.GetUser(ctx, login, password)
	if err != nil {
		return "", err
	}

	return s.generateAccessToken(user.ID)
}

func (s *Service) generateAccessToken(id uuid.UUID) (string, error) {
	token := jwt.New()
	now := time.Now()
	token.Set(jwt.SubjectKey, id.String())
	token.Set(jwt.IssuedAtKey, now.Unix())
	token.Set(jwt.ExpirationKey, now.Add(10*time.Minute))
	signedToken, err := jwt.Sign(token, jwa.HS256, s.secret)
	if err != nil {
		return "", err
	}

	return string(signedToken), nil
}

func (s *Service) fetchOrder(ctx context.Context, orderNumber string) (*model.Order, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(accrualAPIFormat, s.accrualURL, orderNumber), nil)
	if err != nil {
		return nil, err
	}

	res, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusTooManyRequests {
			retryAfter := res.Header.Get("Retry-After")
			ra, err := strconv.ParseInt(retryAfter, 10, 64)
			if err != nil {
				return nil, err
			}
			s.timeout.Store(ra)
			return nil, ErrHittingRateLimit
		}
		return nil, fmt.Errorf("failed to fetch order info, status code %d", res.StatusCode)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var o model.Order
	if err := json.Unmarshal(b, &o); err != nil {
		return nil, err
	}

	return &o, nil
}

func (s *Service) FetchOrders(ctx context.Context, order <-chan model.Order) {
	var wg sync.WaitGroup

	wg.Add(5)
	go s.worker(ctx, &wg, order)
	go s.worker(ctx, &wg, order)
	go s.worker(ctx, &wg, order)
	go s.worker(ctx, &wg, order)
	go s.worker(ctx, &wg, order)
	wg.Wait()
}

func (s *Service) worker(ctx context.Context, wg *sync.WaitGroup, order <-chan model.Order) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Worker has been stopped", slog.String("reason", ctx.Err().Error()))
			return
		default:
			o, ok := <-order
			if !ok {
				return
			}
			o1, err := s.retryForRateLimit(ctx, o.Number, s.fetchOrder)
			if err != nil {
				s.logger.Info("Worker error", slog.String("error", err.Error()))
				continue
			}
			if o1.Status == o.Status {
				continue
			}
			err = s.storage.UpdateOrder(ctx, o.UserID, *o1)
			if err != nil {
				continue
			}
		}
	}
}

func (s *Service) retryForRateLimit(ctx context.Context, order string, fn func(context.Context, string) (*model.Order, error)) (*model.Order, error) {
	o, err := fn(ctx, order)
	if err == nil {
		return o, nil
	}
	if !errors.Is(err, ErrHittingRateLimit) {
		return nil, err
	}
	time.Sleep(time.Duration(s.timeout.Load()) * time.Second)

	return fn(ctx, order)
}
