package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/v-starostin/gophermart/internal/model"
)

const (
	accrualAPIFormat = "%s/api/orders/%d"
)

type Storage interface {
	AddUser(login, password string) error
	GetUser(login, password string) (*model.User, error)
	AddOrder(userID uuid.UUID, order model.Order) error
	GetOrder(userID uuid.UUID, orderNumber int) (*model.Order, error)
	GetOrders(userID uuid.UUID) ([]*model.Order, error)
	WithdrawRequest(userID uuid.UUID, order string, sum int) error
	GetBalance(uuid.UUID) (int, int, error)
	GetWithdrawals(userID uuid.UUID) ([]*model.Withdrawal, error)
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Auth struct {
	storage    Storage
	client     HTTPClient
	secret     []byte
	accrualURL string
}

func New(storage Storage, secret []byte, url string) *Auth {
	return &Auth{
		storage:    storage,
		secret:     secret,
		accrualURL: url,
		client:     &http.Client{},
	}
}

func (a *Auth) GetWithdrawals(userID uuid.UUID) ([]*model.Withdrawal, error) {
	return a.storage.GetWithdrawals(userID)
}

func (a *Auth) GetBalance(userID uuid.UUID) (int, int, error) {
	return a.storage.GetBalance(userID)
}

func (a *Auth) GetOrders(userID uuid.UUID) ([]*model.Order, error) {
	return a.storage.GetOrders(userID)
}

func (a *Auth) WithdrawRequest(userID uuid.UUID, order string, sum int) error {
	return a.storage.WithdrawRequest(userID, order, sum)
}

func (a *Auth) UploadOrder(userID uuid.UUID, number int) error {
	order, err := a.storage.GetOrder(userID, number)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if order != nil {
		return fmt.Errorf("order %d already exists for user %s", number, userID)
	}

	order, err = a.fetchOrder(number)
	if err != nil {
		return err
	}
	return a.storage.AddOrder(userID, *order)
}

func (a *Auth) RegisterUser(login, password string) error {
	return a.storage.AddUser(login, password)
}

func (a *Auth) Authenticate(login, password string) (string, error) {
	user, err := a.storage.GetUser(login, password)
	if err != nil {
		return "", err
	}

	return a.generateAccessToken(user.ID)
}

func (a *Auth) generateAccessToken(id uuid.UUID) (string, error) {
	token := jwt.New()
	now := time.Now()
	token.Set(jwt.SubjectKey, id.String())
	token.Set(jwt.IssuedAtKey, now.Unix())
	token.Set(jwt.ExpirationKey, now.Add(100*time.Minute))
	signedToken, err := jwt.Sign(token, jwa.HS256, a.secret)
	if err != nil {
		return "", err
	}
	return string(signedToken), nil
}

func (a *Auth) fetchOrder(number int) (*model.Order, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(accrualAPIFormat, a.accrualURL, number), nil)
	if err != nil {
		return nil, err
	}
	res, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	log.Println("response status:", res.Status)
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var order model.Order
	if err := json.Unmarshal(b, &order); err != nil {
		return nil, err
	}
	return &order, nil
}
