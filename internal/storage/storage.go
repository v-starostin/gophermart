package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/v-starostin/gophermart/internal/model"
)

type Storage struct {
	db *sql.DB
}

type user struct {
	id       uuid.UUID
	login    string
	password string
}

type order struct {
	id          uuid.UUID
	orderNumber int
	userID      uuid.UUID
	accrual     int
	status      string
	uploadedAt  time.Time
}

func New(db *sql.DB) *Storage {
	return &Storage{db}
}

func (s *Storage) AddOrder(userID uuid.UUID, order model.Order) error {
	orderID, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	query := "INSERT INTO orders (id, user_id, order_number, status, accrual) VALUES ($1,$2,$3,$4,$5)"
	_, err = s.db.Exec(query, orderID, userID, order.Number, order.Status, order.Accrual)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) GetOrder(userID uuid.UUID, orderNumber int) (*model.Order, error) {
	var o order
	query := "SELECT user_id, order_number FROM orders WHERE user_id = $1 AND order = $2"
	if err := s.db.QueryRow(query, userID, orderNumber).Scan(&o.userID, &o.orderNumber); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Storage) AddUser(login, password string) error {
	userID, err := uuid.NewRandom()
	log.Println("id of registered user:", userID)
	if err != nil {
		return err
	}

	_, err = s.db.Exec("INSERT INTO users (id, login, password) VALUES ($1,$2,$3)", userID, login, hash(password))
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) GetUser(login, password string) (*model.User, error) {
	var u user
	query := "SELECT id, login, password FROM users WHERE login = $1 AND password = $2"
	if err := s.db.QueryRow(query, login, hash(password)).Scan(&u.id, &u.login, &u.password); err != nil {
		return nil, err
	}

	return &model.User{
		ID:       u.id,
		Login:    u.login,
		Password: u.password,
	}, nil
}

func hash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
