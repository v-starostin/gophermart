package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/v-starostin/gophermart/internal/model"
)

type Storage struct {
	db *sql.DB
}

func New(db *sql.DB) *Storage {
	return &Storage{db}
}

func (s *Storage) WithdrawRequest(userID uuid.UUID, orderNumber string, sum int) error {
	withdrawID, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	query := "INSERT INTO withdraws (id, user_id, order_number, sum) values ($1,$2,$3,$4)"
	if _, err := s.db.Exec(query, withdrawID, userID, orderNumber, sum); err != nil {
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
		return fmt.Errorf("insufficient funds")
	}
	query = "update balances set balance = $1 where user_id = $2"
	_, err = tx.Exec(query, balance, userID)
	if err != nil {
		return err
	}

	query = "update withdraw_balances set amount = (select amount + $1 from withdraw_balances where user_id=$2) where user_id = $3"
	_, err = tx.Exec(query, sum, userID, userID)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("update withdraws set status = $1 where order_number = $2", "SUCCESS", orderNumber)
	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()

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
	_, err = tx.Exec(query, order.Accrual, userID)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()

	return nil
}

func (s *Storage) GetWithdrawals(userID uuid.UUID) ([]model.Withdrawal, error) {
	query := "select order_number, sum, processed_at from withdraws where user_id = $1 and status = $2"
	raws, err := s.db.Query(query, userID, "SUCCESS")
	if err != nil {
		log.Println("GetWithdrawals1 error:", err.Error())
		return nil, err
	}
	defer raws.Close()

	withdrawals := make([]model.Withdrawal, 0)
	w := model.Withdrawal{}
	for raws.Next() {
		err = raws.Scan(&w.Order, &w.Sum, &w.ProcessedAt)
		if err != nil {
			log.Println("GetWithdrawals2 error:", err.Error())
			return nil, err
		}
		withdrawals = append(withdrawals, w)
	}

	err = raws.Err()
	if err != nil {
		log.Println("GetWithdrawals3 error:", err.Error())
		return nil, err
	}

	return withdrawals, nil
}

func (s *Storage) GetBalance(userID uuid.UUID) (int, int, error) {
	var balance int
	if err := s.db.QueryRow("select balance from balances where user_id = $1", userID).Scan(&balance); err != nil {
		return 0, 0, err
	}

	var withdrawn int
	if err := s.db.QueryRow("select amount from withdraw_balances where user_id = $1", userID).Scan(&withdrawn); err != nil {
		return 0, 0, err
	}

	return balance, withdrawn, nil
}

func (s *Storage) GetOrder(userID uuid.UUID, orderNumber string) (*model.Order, error) {
	var number string
	query := "SELECT order_number FROM orders WHERE user_id = $1 AND order_number = $2"
	if err := s.db.QueryRow(query, userID, orderNumber).Scan(&number); err != nil {
		return nil, err
	}

	return &model.Order{
		Number: number,
	}, nil
}

func (s *Storage) GetOrders(userID uuid.UUID) ([]model.Order, error) {
	query := "select order_number, status, accrual from orders where user_id = $1"
	raws, err := s.db.Query(query, userID)
	if err != nil {
		log.Println("GetOrders1 error:", err.Error())
		return nil, err
	}
	defer raws.Close()

	orders := make([]model.Order, 0)
	o := model.Order{}
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
	var u model.User
	query := "SELECT id, login, password FROM users WHERE login = $1 AND password = $2"
	if err := s.db.QueryRow(query, login, hash(password)).Scan(&u.ID, &u.Login, &u.Password); err != nil {
		return nil, err
	}

	return &u, nil
}

func hash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
