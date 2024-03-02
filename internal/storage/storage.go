package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"

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

func New(db *sql.DB) *Storage {
	return &Storage{db}
}

func (s *Storage) AddUser(login, password string) error {
	userID, err := uuid.NewRandom()
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
