// Package api provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen/v2 version v2.1.0 DO NOT EDIT.
package api

import (
	"time"
)

// Order defines model for Order.
type Order struct {
	Accrual    *string    `json:"accrual,omitempty"`
	Number     *string    `json:"number,omitempty"`
	Status     *string    `json:"status,omitempty"`
	UploadedAt *time.Time `json:"uploaded_at,omitempty"`
}

// User defines model for User.
type User struct {
	Login    *string `json:"login,omitempty"`
	Password *string `json:"password,omitempty"`
}

// LoginUserJSONBody defines parameters for LoginUser.
type LoginUserJSONBody = []User

// UploadOrderJSONBody defines parameters for UploadOrder.
type UploadOrderJSONBody = string

// LoginUserJSONRequestBody defines body for LoginUser for application/json ContentType.
type LoginUserJSONRequestBody = LoginUserJSONBody

// UploadOrderJSONRequestBody defines body for UploadOrder for application/json ContentType.
type UploadOrderJSONRequestBody = UploadOrderJSONBody

// RegisterUserJSONRequestBody defines body for RegisterUser for application/json ContentType.
type RegisterUserJSONRequestBody = User
