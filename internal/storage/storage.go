package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/v-starostin/gophermart/internal/model"
)

var ErrInsufficientBalance = errors.New("insufficient balance")

type Storage struct {
	l  *slog.Logger
	db *sql.DB
}

func New(l *slog.Logger, db *sql.DB) *Storage {
	return &Storage{l, db}
}

func (s *Storage) WithdrawalRequest(ctx context.Context, userID uuid.UUID, orderNumber string, sum float64) error {
	withdrawID, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	query := "INSERT INTO withdrawals (id, user_id, order_number, sum) VALUES ($1, $2, $3, $4)"
	if _, err = s.db.ExecContext(ctx, query, withdrawID, userID, orderNumber, sum); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	var balance float64
	if err = tx.QueryRowContext(ctx, "SELECT balance FROM balances WHERE user_id = $1 FOR UPDATE;", userID).Scan(&balance); err != nil {
		_ = tx.Rollback()
		return err
	}

	balance -= sum
	if balance < 0 {
		_ = tx.Rollback()
		return ErrInsufficientBalance
	}

	if _, err = tx.ExecContext(ctx, "UPDATE balances SET balance = $1 WHERE user_id = $2", balance, userID); err != nil {
		return err
	}

	query = "UPDATE withdraw_balances SET sum = (SELECT sum + $1 FROM withdraw_balances WHERE user_id = $2) WHERE user_id = $3"
	if _, err = tx.ExecContext(ctx, query, sum, userID, userID); err != nil {
		_ = tx.Rollback()
		return err
	}

	if _, err = tx.ExecContext(ctx, "UPDATE withdrawals SET status = $1 WHERE order_number = $2", "SUCCESS", orderNumber); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *Storage) AddOrder(ctx context.Context, userID uuid.UUID, order model.Order) error {
	orderID, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	query := "INSERT INTO orders (id, user_id, order_number, status, accrual) VALUES ($1, $2, $3, $4, $5)"
	if _, err = tx.ExecContext(ctx, query, orderID, userID, order.Number, order.Status, order.Accrual); err != nil {
		_ = tx.Rollback()
		return err
	}

	if order.Status == "PROCESSED" {
		query = "UPDATE balances SET balance = (SELECT balance + $1 FROM balances WHERE user_id = $2) WHERE user_id = $3"
		if _, err = tx.ExecContext(ctx, query, order.Accrual, userID, userID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *Storage) UpdateOrder(ctx context.Context, userID uuid.UUID, order model.Order) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	query := "UPDATE orders SET status = $1, accrual = $2, updated_at = $3 WHERE user_id = $4 AND order_number = $5"
	if _, err = tx.ExecContext(ctx, query, order.Status, order.Accrual, time.Now().UTC(), userID, order.Number); err != nil {
		_ = tx.Rollback()
		return err
	}

	query = "UPDATE balances SET balance = (SELECT balance + $1 FROM balances WHERE user_id = $2) WHERE user_id = $3"
	if _, err = tx.ExecContext(ctx, query, order.Accrual, userID, userID); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *Storage) GetWithdrawals(ctx context.Context, userID uuid.UUID) ([]model.Withdrawal, error) {
	query := "SELECT order_number, sum, processed_at FROM withdrawals WHERE user_id = $1 AND status = $2"
	raws, err := s.db.QueryContext(ctx, query, userID, "SUCCESS")
	if err != nil {
		return nil, err
	}
	defer raws.Close()

	withdrawals := make([]model.Withdrawal, 0)
	w := model.Withdrawal{}
	for raws.Next() {
		err = raws.Scan(&w.Order, &w.Sum, &w.ProcessedAt)
		if err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, w)
	}

	if err = raws.Err(); err != nil {
		return nil, err
	}

	return withdrawals, nil
}

func (s *Storage) GetBalance(ctx context.Context, userID uuid.UUID) (float64, float64, error) {
	var balance float64
	if err := s.db.QueryRowContext(ctx, "SELECT balance FROM balances WHERE user_id = $1", userID).Scan(&balance); err != nil {
		return 0, 0, err
	}

	var withdrawn float64
	if err := s.db.QueryRowContext(ctx, "SELECT sum FROM withdraw_balances WHERE user_id = $1", userID).Scan(&withdrawn); err != nil {
		return 0, 0, err
	}

	return balance, withdrawn, nil
}

func (s *Storage) GetOrder(ctx context.Context, userID uuid.UUID, orderNumber string) (*model.Order, error) {
	var number string
	query := "SELECT order_number FROM orders WHERE user_id = $1 AND order_number = $2"
	if err := s.db.QueryRowContext(ctx, query, userID, orderNumber).Scan(&number); err != nil {
		return nil, err
	}

	return &model.Order{
		Number: number,
	}, nil
}

func (s *Storage) GetOrders(ctx context.Context, userID uuid.UUID) ([]model.Order, error) {
	query := "SELECT order_number, status, accrual, uploaded_at FROM orders WHERE user_id = $1"
	raws, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer raws.Close()

	orders := make([]model.Order, 0)
	o := model.Order{}
	for raws.Next() {
		err = raws.Scan(&o.Number, &o.Status, &o.Accrual, &o.UploadedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}

	if err = raws.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (s *Storage) AddUser(ctx context.Context, login, password string) error {
	userID, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	_, err = tx.Exec("INSERT INTO users (id, login, password) VALUES ($1, $2, $3)", userID, login, hash(password))
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	_, err = tx.Exec("INSERT INTO balances (user_id) VALUES ($1)", userID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	_, err = tx.Exec("INSERT INTO withdraw_balances (user_id) VALUES ($1)", userID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *Storage) GetUser(ctx context.Context, login, password string) (*model.User, error) {
	var u model.User
	query := "SELECT id, login, password FROM users WHERE login = $1 AND password = $2"
	if err := s.db.QueryRowContext(ctx, query, login, hash(password)).Scan(&u.ID, &u.Login, &u.Password); err != nil {
		return nil, err
	}

	return &u, nil
}

func hash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (s *Storage) ScanOrders(ctx context.Context) <-chan model.Order {
	ch := make(chan model.Order)
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-time.After(5 * time.Second):
				s.l.Info("Scanning orders")
				query := "SELECT order_number, status, user_id FROM orders WHERE status != $1 AND status != $2"
				raws, err := s.db.QueryContext(ctx, query, "PROCESSED", "INVALID")
				if err != nil {
					s.l.Info("Scanning orders error", slog.String("error", err.Error()))
					continue
				}
				defer raws.Close()

				var o model.Order
				for raws.Next() {
					if err = raws.Scan(&o.Number, &o.Status, &o.UserID); err != nil {
						s.l.Info("Scanning orders error", slog.String("error", err.Error()))
						continue
					}
					ch <- o
				}
				if err = raws.Err(); err != nil {
					s.l.Info("Scanning orders error", slog.String("error", err.Error()))
					continue
				}
			case <-ctx.Done():
				s.l.Info("Scanning orders stopped")
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		s.l.Info("Closing channel")
		close(ch)
	}()

	return ch
}
