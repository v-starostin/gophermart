package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"

	"github.com/google/uuid"
)

type Storage struct {
	db *sql.DB
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

func (s *Storage) GetUser(login string) error { return nil }

func hash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
