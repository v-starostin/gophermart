package service

import (
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/v-starostin/gophermart/internal/model"
)

type Storage interface {
	AddUser(login, password string) error
	GetUser(login, password string) (*model.User, error)
}

type Auth struct {
	storage Storage
	secret  []byte
}

func New(storage Storage, secret []byte) *Auth {
	return &Auth{storage, secret}
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
	token.Set(jwt.ExpirationKey, now.Add(10*time.Minute))
	signedToken, err := jwt.Sign(token, jwa.HS256, a.secret)
	if err != nil {
		return "", err
	}
	return string(signedToken), nil
}
