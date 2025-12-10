package db

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/citizenwallet/engine/pkg/common"
	"github.com/citizenwallet/engine/pkg/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserOpStatus represents the status of a user operation
type UserOpStatus string

const (
	UserOpStatusPending   UserOpStatus = "pending"
	UserOpStatusSubmitted UserOpStatus = "submitted"
	UserOpStatusSuccess   UserOpStatus = "success"
	UserOpStatusReverted  UserOpStatus = "reverted"
	UserOpStatusTimeout   UserOpStatus = "timeout"
)

// StoredUserOp represents a persisted user operation
type StoredUserOp struct {
	UserOpHash string           `json:"user_op_hash"`
	TxHash     *string          `json:"tx_hash"`
	Status     UserOpStatus     `json:"status"`
	ValidUntil int64            `json:"valid_until"`
	ValidAfter int64            `json:"valid_after"`
	Sender     string           `json:"sender"`
	Paymaster  string           `json:"paymaster"`
	EntryPoint string           `json:"entry_point"`
	UserOp     *json.RawMessage `json:"user_op"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

type UserOpDB struct {
	ctx    context.Context
	suffix string
	db     *pgxpool.Pool
	rdb    *pgxpool.Pool
}

// NewUserOpDB creates a new UserOpDB
func NewUserOpDB(ctx context.Context, db, rdb *pgxpool.Pool, name string) (*UserOpDB, error) {
	udb := &UserOpDB{
		ctx:    ctx,
		suffix: name,
		db:     db,
		rdb:    rdb,
	}

	return udb, nil
}

// CreateUserOpTable creates the user operations table
func (db *UserOpDB) CreateUserOpTable() error {
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS t_userops_%s(
		user_op_hash TEXT NOT NULL PRIMARY KEY,
		tx_hash TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		valid_until BIGINT NOT NULL,
		valid_after BIGINT NOT NULL,
		sender TEXT NOT NULL,
		paymaster TEXT NOT NULL,
		entry_point TEXT NOT NULL,
		user_op JSONB NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
		updated_at TIMESTAMP NOT NULL DEFAULT current_timestamp
	);
	`, db.suffix))

	return err
}

// CreateUserOpTableIndexes creates indexes for the user operations table
func (db *UserOpDB) CreateUserOpTableIndexes() error {
	suffix := common.ShortenName(db.suffix, 6)

	// Index on status for querying pending/submitted ops
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	CREATE INDEX IF NOT EXISTS idx_userops_%s_status ON t_userops_%s (status);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	// Index on valid_after for ordering retries
	_, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	CREATE INDEX IF NOT EXISTS idx_userops_%s_valid_after ON t_userops_%s (valid_after);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	// Index on tx_hash for looking up by transaction
	_, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	CREATE INDEX IF NOT EXISTS idx_userops_%s_tx_hash ON t_userops_%s (tx_hash);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	// Composite index for pending ops ordered by valid_after
	_, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	CREATE INDEX IF NOT EXISTS idx_userops_%s_status_valid_after ON t_userops_%s (status, valid_after);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	return nil
}

// AddUserOp adds a new user operation to the database
func (db *UserOpDB) AddUserOp(userOpHash string, validUntil, validAfter int64, sender, paymaster, entryPoint string, userOp *engine.UserOp) error {
	userOpJSON, err := json.Marshal(userOp)
	if err != nil {
		return err
	}

	_, err = db.db.Exec(db.ctx, fmt.Sprintf(`
	INSERT INTO t_userops_%s (user_op_hash, status, valid_until, valid_after, sender, paymaster, entry_point, user_op, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	ON CONFLICT (user_op_hash) DO NOTHING
	`, db.suffix), userOpHash, UserOpStatusPending, validUntil, validAfter, sender, paymaster, entryPoint, userOpJSON, time.Now().UTC(), time.Now().UTC())

	return err
}

// UpdateStatus updates the status of a user operation
func (db *UserOpDB) UpdateStatus(userOpHash string, status UserOpStatus) error {
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	UPDATE t_userops_%s SET status = $1, updated_at = $2 WHERE user_op_hash = $3
	`, db.suffix), status, time.Now().UTC(), userOpHash)

	return err
}

// UpdateTxHash updates the transaction hash of a user operation
func (db *UserOpDB) UpdateTxHash(userOpHash, txHash string) error {
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	UPDATE t_userops_%s SET tx_hash = $1, updated_at = $2 WHERE user_op_hash = $3
	`, db.suffix), txHash, time.Now().UTC(), userOpHash)

	return err
}

// UpdateStatusAndTxHash updates both the status and transaction hash
func (db *UserOpDB) UpdateStatusAndTxHash(userOpHash string, status UserOpStatus, txHash string) error {
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	UPDATE t_userops_%s SET status = $1, tx_hash = $2, updated_at = $3 WHERE user_op_hash = $4
	`, db.suffix), status, txHash, time.Now().UTC(), userOpHash)

	return err
}

// GetUserOp retrieves a user operation by its hash
func (db *UserOpDB) GetUserOp(userOpHash string) (*StoredUserOp, error) {
	var op StoredUserOp

	err := db.rdb.QueryRow(db.ctx, fmt.Sprintf(`
	SELECT user_op_hash, tx_hash, status, valid_until, valid_after, sender, paymaster, entry_point, user_op, created_at, updated_at
	FROM t_userops_%s
	WHERE user_op_hash = $1
	`, db.suffix), userOpHash).Scan(
		&op.UserOpHash,
		&op.TxHash,
		&op.Status,
		&op.ValidUntil,
		&op.ValidAfter,
		&op.Sender,
		&op.Paymaster,
		&op.EntryPoint,
		&op.UserOp,
		&op.CreatedAt,
		&op.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &op, nil
}

// GetPendingByValidAfter retrieves pending user operations ordered by valid_after (smallest first)
func (db *UserOpDB) GetPendingByValidAfter(limit int) ([]*StoredUserOp, error) {
	rows, err := db.rdb.Query(db.ctx, fmt.Sprintf(`
	SELECT user_op_hash, tx_hash, status, valid_until, valid_after, sender, paymaster, entry_point, user_op, created_at, updated_at
	FROM t_userops_%s
	WHERE status = $1 AND valid_until > $2
	ORDER BY valid_after ASC
	LIMIT $3
	`, db.suffix), UserOpStatusPending, time.Now().Unix(), limit)
	if err != nil {
		if err == pgx.ErrNoRows {
			return []*StoredUserOp{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var ops []*StoredUserOp
	for rows.Next() {
		var op StoredUserOp
		err := rows.Scan(
			&op.UserOpHash,
			&op.TxHash,
			&op.Status,
			&op.ValidUntil,
			&op.ValidAfter,
			&op.Sender,
			&op.Paymaster,
			&op.EntryPoint,
			&op.UserOp,
			&op.CreatedAt,
			&op.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		ops = append(ops, &op)
	}

	return ops, nil
}

// GetExpiredPending retrieves pending user operations that have expired (validUntil < now)
func (db *UserOpDB) GetExpiredPending() ([]*StoredUserOp, error) {
	rows, err := db.rdb.Query(db.ctx, fmt.Sprintf(`
	SELECT user_op_hash, tx_hash, status, valid_until, valid_after, sender, paymaster, entry_point, user_op, created_at, updated_at
	FROM t_userops_%s
	WHERE status = $1 AND valid_until <= $2
	`, db.suffix), UserOpStatusPending, time.Now().Unix())
	if err != nil {
		if err == pgx.ErrNoRows {
			return []*StoredUserOp{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var ops []*StoredUserOp
	for rows.Next() {
		var op StoredUserOp
		err := rows.Scan(
			&op.UserOpHash,
			&op.TxHash,
			&op.Status,
			&op.ValidUntil,
			&op.ValidAfter,
			&op.Sender,
			&op.Paymaster,
			&op.EntryPoint,
			&op.UserOp,
			&op.CreatedAt,
			&op.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		ops = append(ops, &op)
	}

	return ops, nil
}

// MarkExpiredAsReverted marks all expired pending user operations as reverted
func (db *UserOpDB) MarkExpiredAsReverted() error {
	_, err := db.db.Exec(db.ctx, fmt.Sprintf(`
	UPDATE t_userops_%s SET status = $1, updated_at = $2 
	WHERE status = $3 AND valid_until <= $4
	`, db.suffix), UserOpStatusReverted, time.Now().UTC(), UserOpStatusPending, time.Now().Unix())

	return err
}

// GetUserOpByTxHash retrieves a user operation by its transaction hash
func (db *UserOpDB) GetUserOpByTxHash(txHash string) (*StoredUserOp, error) {
	var op StoredUserOp

	err := db.rdb.QueryRow(db.ctx, fmt.Sprintf(`
	SELECT user_op_hash, tx_hash, status, valid_until, valid_after, sender, paymaster, entry_point, user_op, created_at, updated_at
	FROM t_userops_%s
	WHERE tx_hash = $1
	`, db.suffix), txHash).Scan(
		&op.UserOpHash,
		&op.TxHash,
		&op.Status,
		&op.ValidUntil,
		&op.ValidAfter,
		&op.Sender,
		&op.Paymaster,
		&op.EntryPoint,
		&op.UserOp,
		&op.CreatedAt,
		&op.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &op, nil
}

// GetValidUntilValidAfter retrieves the validity period for a user operation
func (db *UserOpDB) GetValidUntilValidAfter(userOpHash string) (validUntil, validAfter *big.Int, err error) {
	var vUntil, vAfter int64

	err = db.rdb.QueryRow(db.ctx, fmt.Sprintf(`
	SELECT valid_until, valid_after
	FROM t_userops_%s
	WHERE user_op_hash = $1
	`, db.suffix), userOpHash).Scan(&vUntil, &vAfter)

	if err != nil {
		return nil, nil, err
	}

	return big.NewInt(vUntil), big.NewInt(vAfter), nil
}
