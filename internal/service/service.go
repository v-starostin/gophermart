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

	"github.com/v-starostin/gophermart/internal/currency"
	"github.com/v-starostin/gophermart/internal/model"
)

const (
	accrualAPIFormat = "%s/api/orders/%s"
)

var (
	ErrOrderAlreadyExists = errors.New("order already exists for current user")
)

type Storage interface {
	AddUser(login, password string) error
	GetUser(login, password string) (*model.User, error)
	AddOrder(userID uuid.UUID, order model.Order) error
	GetOrder(userID uuid.UUID, orderNumber string) (*model.Order, error)
	GetOrders(userID uuid.UUID) ([]model.Order, error)
	WithdrawRequest(userID uuid.UUID, orderNumber string, sum int) error
	GetBalance(uuid.UUID) (int, int, error)
	GetWithdrawals(userID uuid.UUID) ([]model.Withdrawal, error)
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

func (a *Auth) GetWithdrawals(userID uuid.UUID) ([]model.Withdrawal, error) {
	return a.storage.GetWithdrawals(userID)
}

func (a *Auth) GetBalance(userID uuid.UUID) (float64, float64, error) {
	balance, withdraw, err := a.storage.GetBalance(userID)
	if err != nil {
		return 0, 0, err
	}

	return currency.ConvertToPrimary(balance), currency.ConvertToPrimary(withdraw), nil
}

func (a *Auth) GetOrders(userID uuid.UUID) ([]model.Order, error) {
	return a.storage.GetOrders(userID)
}

func (a *Auth) WithdrawRequest(userID uuid.UUID, orderNumber string, sum float64) error {
	return a.storage.WithdrawRequest(userID, orderNumber, currency.ConvertToSubunit(sum))
}

func (a *Auth) UploadOrder(userID uuid.UUID, orderNumber string) error {
	_, err := a.storage.GetOrder(userID, orderNumber)
	if err == nil {
		return ErrOrderAlreadyExists
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	o, err := a.fetchOrder(orderNumber)
	if err != nil {
		return err
	}

	order := model.Order{
		Number:  o.Number,
		Status:  o.Status,
		Accrual: currency.ConvertToSubunit(o.Accrual),
	}

	return a.storage.AddOrder(userID, order)
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

func (a *Auth) fetchOrder(orderNumber string) (*order, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(accrualAPIFormat, a.accrualURL, orderNumber), nil)
	if err != nil {
		return nil, err
	}
	res, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch order info, status code %d", res.StatusCode)
	}
	log.Println("response status:", res.Status)
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	log.Printf("response: %v\n", string(b))
	var o order
	if err := json.Unmarshal(b, &o); err != nil {
		log.Println("fetchOrder:", err.Error())
		return nil, err
	}
	return &o, nil
}

type order struct {
	Number     string    `json:"order"`
	Status     string    `json:"status"`
	Accrual    float64   `json:"accrual"`
	UploadedAt time.Time `json:"uploaded_at"`
}
