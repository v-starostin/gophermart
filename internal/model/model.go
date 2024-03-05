package model

import (
	"time"

	"github.com/google/uuid"
)

type Error struct {
	Error string `json:"error"`
}

type User struct {
	ID       uuid.UUID
	Login    string
	Password string
}

type AccrualOrder struct {
	Order   string `json:"order"`
	Status  string `json:"status"`
	Accrual int    `json:"accrual"`
}

type Order struct {
	Number     string    `json:"order"`
	Status     string    `json:"status"`
	Accrual    int       `json:"accrual"`
	UploadedAt time.Time `json:"uploaded_at"`
}
