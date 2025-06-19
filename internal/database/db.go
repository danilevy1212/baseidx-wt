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
	CREATE TABLE transactions (
    	hash TEXT PRIMARY KEY,
    	type TEXT NOT NULL CHECK (type IN ('transfer', 'call', 'fee')),
    	value NUMERIC NOT NULL,
    	from_address TEXT NOT NULL,
    	to_address TEXT,
    	block_index TEXT NOT NULL,
    	succesful BOOLEAN NOT NULL,
    	timestamp TIMESTAMPTZ NOT NULL
	);

	CREATE TABLE fees (
    	transaction_hash TEXT PRIMARY KEY REFERENCES transactions(hash) ON DELETE CASCADE,
    	amount NUMERIC NOT NULL,
    	from_address TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_fees_from_address ON fees(from_address);
	CREATE INDEX IF NOT EXISTS idx_transactions_from ON transactions(from_address);
	CREATE INDEX IF NOT EXISTS idx_transactions_to ON transactions(to_address);
	CREATE INDEX IF NOT EXISTS idx_transactions_from_timestamp ON transactions(from_address, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_transactions_to_timestamp ON transactions(to_address, timestamp DESC);
	`)

	return err
}

func (db *DBClient) UpsertTransaction(ctx context.Context, tx Transaction) error {
	_, err := db.Conn.Exec(ctx, `
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

	return err
}

func (db *DBClient) UpsertFee(ctx context.Context, fee Fee) error {
	_, err := db.Conn.Exec(ctx, `
	INSERT INTO fees (transaction_hash, amount, from_address)
	VALUES ($1, $2, $3)
	ON CONFLICT (transaction_hash) DO UPDATE SET
		amount = EXCLUDED.amount,
		from_address = EXCLUDED.from_address;
	`, fee.TransactionHash, fee.Amount, fee.FromAddress)

	return err
}
