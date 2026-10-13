package balances

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Transaction struct {
	ID           uuid.UUID `json:"id"`
	OwnerLogin   string    `json:"ownerLogin"`
	Amount       float64   `json:"amount"`
	BalanceAfter float64   `json:"balanceAfter"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Repository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{pool: pool, logger: logger}
}

func (r *Repository) GetBalance(ctx context.Context, owner string) (float64, error) {
	var balance float64
	err := r.pool.QueryRow(ctx, `SELECT balance FROM users WHERE login = $1`, owner).Scan(&balance)
	if err != nil {
		return 0, err
	}
	return balance, nil
}

func (r *Repository) AddBalance(ctx context.Context, owner string, amount float64, reason string) (float64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var newBalance float64
	err = tx.QueryRow(ctx, `
		UPDATE users SET balance = balance + $1, updated_at = now() WHERE login = $2 RETURNING balance
	`, amount, owner).Scan(&newBalance)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO balance_transactions (id, owner_login, amount, balance_after, reason)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), owner, amount, newBalance, reason)
	if err != nil {
		return 0, err
	}

	return newBalance, tx.Commit(ctx)
}

func (r *Repository) DeductBalance(ctx context.Context, owner string, amount float64, reason string) (float64, error) {
	return r.AddBalance(ctx, owner, -amount, reason)
}

func (r *Repository) ListTransactions(ctx context.Context, owner string, limit, offset int) ([]Transaction, int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, owner_login, amount, balance_after, reason, created_at
		FROM balance_transactions WHERE owner_login = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`, owner, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.OwnerLogin, &t.Amount, &t.BalanceAfter, &t.Reason, &t.CreatedAt); err != nil {
			return nil, 0, err
		}
		transactions = append(transactions, t)
	}
	if transactions == nil {
		transactions = []Transaction{}
	}

	var total int64
	_ = r.pool.QueryRow(ctx, `SELECT count(*) FROM balance_transactions WHERE owner_login = $1`, owner).Scan(&total)

	return transactions, total, rows.Err()
}

func (r *Repository) ListAllTransactions(ctx context.Context, limit, offset int) ([]Transaction, int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, owner_login, amount, balance_after, reason, created_at
		FROM balance_transactions ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.OwnerLogin, &t.Amount, &t.BalanceAfter, &t.Reason, &t.CreatedAt); err != nil {
			return nil, 0, err
		}
		transactions = append(transactions, t)
	}
	if transactions == nil {
		transactions = []Transaction{}
	}

	var total int64
	_ = r.pool.QueryRow(ctx, `SELECT count(*) FROM balance_transactions`).Scan(&total)

	return transactions, total, rows.Err()
}

func (r *Repository) SetBalance(ctx context.Context, owner string, balance float64) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET balance = $1, updated_at = now() WHERE login = $2`, balance, owner)
	return err
}