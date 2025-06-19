package database

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/danilevy1212/baseidx-wt/internal/config"
)

type DBClient struct {
	Conn *pgx.Conn
}

func New(ctx context.Context, c config.DBConfig) (*DBClient, error) {
	conn, err := pgx.Connect(ctx, c.String())

	if err != nil {
		return nil, err
	}

	return &DBClient{
		Conn: conn,
	}, nil
}

func (db *DBClient) Close(ctx context.Context) error {
	return db.Conn.Close(ctx)
}

func (db *DBClient) Ping(ctx context.Context) error {
	return db.Conn.Ping(ctx)
}

func (db *DBClient) CreateSchema(ctx context.Context) error {
	_, err := db.Conn.Exec(ctx, `
	CREATE TABLE transactions (
    	hash TEXT PRIMARY KEY,
    	type TEXT NOT NULL CHECK (type IN ('transfer', 'call', 'fee')),
    	value NUMERIC NOT NULL,
    	from_address TEXT NOT NULL,
    	to_address TEXT NOT NULL,
    	block_index TEXT NOT NULL,
    	succesful BOOLEAN NOT NULL,
    	timestamp TIMESTAMPTZ NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_transactions_from ON transactions(from_address);
	CREATE INDEX IF NOT EXISTS idx_transactions_to ON transactions(to_address);
	CREATE INDEX IF NOT EXISTS idx_transactions_from_timestamp ON transactions(from_address, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_transactions_to_timestamp ON transactions(to_address, timestamp DESC);
	`)

	return err
}

func (db *DBClient) UpsertTransactions(ctx context.Context, txs []Transaction) error {
	if len(txs) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for _, tx := range txs {
		batch.Queue(`
			INSERT INTO transactions (hash, type, value, from_address, to_address, block_index, succesful, timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (hash) DO UPDATE SET
				type = EXCLUDED.type,
				value = EXCLUDED.value,
				from_address = EXCLUDED.from_address,
				to_address = EXCLUDED.to_address,
				block_index = EXCLUDED.block_index,
				succesful = EXCLUDED.succesful,
				timestamp = EXCLUDED.timestamp;
		`, tx.Hash, tx.Type, tx.Value, tx.From, tx.To, tx.BlockIndex, tx.Succesful, tx.Timestamp)
	}

	br := db.Conn.SendBatch(ctx, batch)
	defer br.Close()

	for range txs {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}

	return nil
}

type GetBalanceResult struct {
	Balance      decimal.Decimal
	Transactions uint64
}

func (db *DBClient) GetBalance(ctx context.Context, address string) (GetBalanceResult, error) {
	var result GetBalanceResult

	log.Printf("Getting balance for address %s", address)

	err := db.Conn.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(
				CASE
					WHEN type = 'fee' AND from_address = $1 THEN -value
					WHEN succesful = TRUE AND from_address = $1 THEN -value
					WHEN succesful = TRUE AND to_address   = $1 THEN  value
					ELSE 0
				END
			), 0),
			COUNT(*)
		FROM transactions
		WHERE from_address = $1 OR to_address = $1;
	`, address).Scan(&result.Balance, &result.Transactions)

	if err != nil {
		log.Printf("Error getting balance for address %s: %v", address, err)
		return GetBalanceResult{}, err
	}

	return result, nil
}

func (db *DBClient) GetTransactionsFromAddress(ctx context.Context, address string) ([]Transaction, error) {
	rows, err := db.Conn.Query(ctx, `
		SELECT hash, type, value, from_address, to_address, block_index, succesful, timestamp AT TIME ZONE 'UTC'
		FROM transactions
		WHERE from_address = $1 OR to_address = $1
		ORDER BY timestamp DESC;
	`, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	txs := []Transaction{}

	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(
			&tx.Hash, &tx.Type, &tx.Value, &tx.From, &tx.To,
			&tx.BlockIndex, &tx.Succesful, &tx.Timestamp,
		); err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return txs, nil
}

func (db *DBClient) GetTransactionsInRange(ctx context.Context, start, end time.Time) ([]Transaction, error) {
	rows, err := db.Conn.Query(ctx, `
		SELECT hash, type, value, from_address, to_address, block_index, succesful, timestamp AT TIME ZONE 'UTC'
		FROM transactions
		WHERE timestamp >= $1 AND timestamp <= $2
		ORDER BY timestamp DESC;
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	txs := []Transaction{}
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(
			&tx.Hash, &tx.Type, &tx.Value, &tx.From, &tx.To,
			&tx.BlockIndex, &tx.Succesful, &tx.Timestamp,
		); err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return txs, nil
}
