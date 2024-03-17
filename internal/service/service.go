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
	ErrNoContent          = errors.New("no content")
	ErrOrderAlreadyExists = errors.New("order already exists for current user")
	ErrHittingRateLimit   = errors.New("hitting rate limit")
)

var (
	timeout time.Duration = 0
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
	var t time.Duration
	o, t, err = s.fetchOrder(ctx, orderNumber)
	if err != nil {
		s.logger.Info("Failed to fetch order", slog.String("error", err.Error()))
		if errors.Is(err, ErrHittingRateLimit) {
			timeout = t
		}
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

func (s *Service) fetchOrder(ctx context.Context, orderNumber string) (*model.Order, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(accrualAPIFormat, s.accrualURL, orderNumber), nil)
	if err != nil {
		return nil, 0, err
	}

	res, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusNoContent {
			return nil, 0, ErrNoContent
		}
		if res.StatusCode == http.StatusTooManyRequests {
			retryAfter := res.Header.Get("Retry-After")
			ra, err := strconv.ParseInt(retryAfter, 10, 64)
			if err != nil {
				return nil, 0, err
			}
			return nil, time.Duration(ra), ErrHittingRateLimit
		}
		return nil, 0, fmt.Errorf("failed to fetch order info, status code %d", res.StatusCode)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, 0, err
	}

	var o model.Order
	if err := json.Unmarshal(b, &o); err != nil {
		return nil, 0, err
	}

	return &o, 0, nil
}

func (s *Service) FetchOrders(ctx context.Context, order <-chan model.Order) {
	var wg sync.WaitGroup
	//var timeout time.Duration

	wg.Add(5)
	go s.worker(ctx, 1, &wg, order, &timeout)
	go s.worker(ctx, 2, &wg, order, &timeout)
	go s.worker(ctx, 3, &wg, order, &timeout)
	go s.worker(ctx, 4, &wg, order, &timeout)
	go s.worker(ctx, 5, &wg, order, &timeout)
	wg.Wait()
}

func (s *Service) worker(ctx context.Context, id int, wg *sync.WaitGroup, order <-chan model.Order, timeout *time.Duration) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Worker has been stopped", slog.String("reason", ctx.Err().Error()))
		default:
			o, ok := <-order
			if !ok {
				return
			}
			s.logger.Info("Received from channel", slog.Any("order", o))
			time.Sleep(*timeout + time.Duration(id*100)*time.Millisecond)
			s.logger.Info("Waiting", slog.String("timeout", (*timeout).String()))
			o1, t, err := s.fetchOrder(ctx, o.Number)
			if err != nil && !errors.Is(err, ErrHittingRateLimit) {
				s.logger.Info("Worker", slog.String("error", err.Error()))
				continue
			}
			s.logger.Info("Fetched order", slog.Any("order", o1))
			if t > 0 {
				*timeout = t
			} else {
				*timeout = time.Duration(0)
			}
			if o1.Status == o.Status {
				s.logger.Info("Order statuses are equal", slog.String("db", o.Status), slog.String("fetched", o1.Status))
				continue
			}
			err = s.storage.UpdateOrder(ctx, o.UserID, *o1)
			if err != nil {
				continue
			}
		}
	}
}
