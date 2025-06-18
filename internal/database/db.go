package database

import (
	"context"

	"github.com/jackc/pgx/v5"

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
	CREATE TABLE IF NOT EXISTS transactions (
		hash TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		value NUMERIC NOT NULL,
		from_address TEXT NOT NULL,
		to_address TEXT NOT NULL,
		block_index TEXT NOT NULL,
		succesful BOOLEAN NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		account_address TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS fees (
		transaction_hash TEXT PRIMARY KEY REFERENCES transactions(hash) ON DELETE CASCADE,
		amount NUMERIC NOT NULL
	);

	CREATE TABLE IF NOT EXISTS accounts (
		address TEXT PRIMARY KEY,
		balance NUMERIC NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_transactions_account_timestamp
  	  ON transactions (account_address, timestamp DESC);

	ALTER TABLE fees
  	  ADD CONSTRAINT fk_fees_transaction
  	  FOREIGN KEY (transaction_hash) REFERENCES transactions(hash)
  	  ON DELETE CASCADE;

	CREATE UNIQUE INDEX IF NOT EXISTS idx_fees_transaction_hash ON fees(transaction_hash);
	`)

	return err
}
