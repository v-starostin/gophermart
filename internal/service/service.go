package service

type Storage interface {
	AddUser(login, password string) error
}
type Auth struct {
	storage Storage
}

func New(s Storage) *Auth {
	return &Auth{s}
}

func (a *Auth) AddUser(login, password string) error {
	return a.storage.AddUser(login, password)
}
