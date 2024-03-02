package api

import (
	"context"
)

type Service interface {
	AddUser(username, password string) error
}

type Gophermart struct {
	service Service
}

var _ StrictServerInterface = (*Gophermart)(nil)

func NewGophermart(s Service) *Gophermart {
	return &Gophermart{service: s}
}

func (g *Gophermart) RegisterUser(ctx context.Context, request RegisterUserRequestObject) (RegisterUserResponseObject, error) {
	if err := g.service.AddUser(*request.Body.Login, *request.Body.Password); err != nil {
		return nil, err
	}
	return RegisterUser200JSONResponse{}, nil
}

func (g *Gophermart) LoginUser(ctx context.Context, request LoginUserRequestObject) (LoginUserResponseObject, error) {
	return nil, nil
}

func (g *Gophermart) GetOrders(ctx context.Context, request GetOrdersRequestObject) (GetOrdersResponseObject, error) {
	return nil, nil
}

func (g *Gophermart) UploadOrder(ctx context.Context, request UploadOrderRequestObject) (UploadOrderResponseObject, error) {
	return nil, nil
}

func (g *Gophermart) Ping(ctx context.Context, request PingRequestObject) (PingResponseObject, error) {
	return Ping200JSONResponse("pong"), nil
}
