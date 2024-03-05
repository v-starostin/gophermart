package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
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

func (s *Storage) WithdrawRequest(userID uuid.UUID, order string, sum int) error {
	withdrawID, _ := uuid.NewRandom()

	query := "insert into withdraws (id, user_id, order_id, sum, status) values ($1,$2,$3,$4,$5)"
	if _, err := s.db.Exec(query, withdrawID, userID, order, sum); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	var balance int
	query = "select balance from balances where user_id = $1"
	err = tx.QueryRow(query, userID).Scan(&balance)
	if err != nil {
		tx.Rollback()
		return err
	}

	balance -= sum
	if balance < 0 {
		tx.Rollback()
		s.db.Exec("update withdraws set status = $1 where order_id = $2", "failure", order)
		return fmt.Errorf("err")
	}
	query = "update balances set balance = $1 where user_id = $2"
	_, err = tx.Exec(query, balance, userID)
	if err != nil {
		return err
	}

	query = "update withdraw_balances set amount = (select amount + $1 from whithdraw_balances where user_id=$2) where user_id = $3"
	_, err = tx.Exec(query, sum, userID, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec("update withdraws set status = $1 where order_id = $2", "success", order)
	if err != nil {
		return err
	}
	return nil
}

func (s *Storage) AddOrder(userID uuid.UUID, order model.Order) error {
	orderID, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	query := "INSERT INTO orders (id, user_id, order_number, status, accrual) VALUES ($1,$2,$3,$4,$5)"
	_, err = tx.Exec(query, orderID, userID, order.Number, order.Status, order.Accrual)
	if err != nil {
		tx.Rollback()
		return err
	}

	query = "update balances set balance = (select sum(balance) + $1 from balances where user_id=$2)"
	_, err = tx.Exec(query, order.Accrual)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()

	return nil
}

func (s *Storage) GetBalance(userID uuid.UUID) (int, int, error) {
	var balance int
	if err := s.db.QueryRow("select balance from accounts where user_id = $1").Scan(&balance); err != nil {
		return 0, 0, err
	}

	var withdrawn int
	if err := s.db.QueryRow("select amount from whithdraw_balances where user_id = $1").Scan(&balance); err != nil {
		return 0, 0, err
	}

	return balance, withdrawn, nil
}

func (s *Storage) GetOrder(userID uuid.UUID, orderNumber int) (*model.Order, error) {
	var o order
	query := "SELECT user_id, order_number FROM orders WHERE user_id = $1 AND order = $2"
	if err := s.db.QueryRow(query, userID, orderNumber).Scan(&o.userID, &o.orderNumber); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Storage) GetOrders(userID uuid.UUID) ([]*model.Order, error) {
	query := "select order_number, status, accrual from orders where user_id = $1"
	raws, err := s.db.Query(query, userID)
	if err != nil {
		log.Println("GetOrders1 error:", err.Error())
		return nil, err
	}
	defer raws.Close()

	orders := make([]*model.Order, 0)
	o := &model.Order{}
	for raws.Next() {
		err = raws.Scan(&o.Number, &o.Status, &o.Accrual)
		if err != nil {
			log.Println("GetOrders2 error:", err.Error())
			return nil, err
		}
		orders = append(orders, o)
	}

	err = raws.Err()
	if err != nil {
		log.Println("GetOrders3 error:", err.Error())
		return nil, err
	}

	return orders, nil
}

func (s *Storage) AddUser(login, password string) error {
	userID, err := uuid.NewRandom()
	log.Println("id of registered user:", userID)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO users (id, login, password) VALUES ($1,$2,$3)", userID, login, hash(password))
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("insert into balances (user_id) values ($1)", userID)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("insert into withdraw_balances (user_id) values ($1)", userID)
	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()

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
