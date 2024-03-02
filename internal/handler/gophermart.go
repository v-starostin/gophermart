package handler

import (
	"context"

	"github.com/v-starostin/gophermart/internal/api"
)

type Gophermart struct{}

var _ api.StrictServerInterface = (*Gophermart)(nil)

func NewGophermart() *Gophermart {
	return &Gophermart{}
}

func (g *Gophermart) RegisterUser(ctx context.Context, request api.RegisterUserRequestObject) (api.RegisterUserResponseObject, error) {
	return nil, nil
}

func (g *Gophermart) LoginUser(ctx context.Context, request api.LoginUserRequestObject) (api.LoginUserResponseObject, error) {
	return nil, nil
}

func (g *Gophermart) GetOrders(ctx context.Context, request api.GetOrdersRequestObject) (api.GetOrdersResponseObject, error) {
	return nil, nil
}

func (g *Gophermart) UploadOrder(ctx context.Context, request api.UploadOrderRequestObject) (api.UploadOrderResponseObject, error) {
	return nil, nil
}

func (g *Gophermart) Ping(ctx context.Context, request api.PingRequestObject) (api.PingResponseObject, error) {
	return api.Ping200JSONResponse("pong"), nil
}
