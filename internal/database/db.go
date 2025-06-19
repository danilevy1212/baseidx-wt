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

type GetBalanceResult struct {
	Address      string          `json:"address"`
	Balance      decimal.Decimal `json:"balance"`
	Transactions uint64          `json:"transactions"` // Number of transactions involving this address
}

func (db *DBClient) GetBalance(ctx context.Context, address string) (GetBalanceResult, error) {
	var balance decimal.Decimal
	var count uint64

	log.Printf("Getting balance for address %s", address)

	err := db.Conn.QueryRow(ctx, `
		WITH relevant AS (
			SELECT *
			FROM transactions
			WHERE from_address = $1 OR to_address = $1
		)
		SELECT
			COALESCE(SUM(CASE
				WHEN succesful = TRUE AND from_address = $1 THEN -value
				WHEN succesful = TRUE AND to_address   = $1 THEN  value
				ELSE 0
			END), 0)
			-
			COALESCE((
				SELECT SUM(amount)
				FROM fees
				WHERE from_address = $1
			), 0) AS balance,
			COUNT(*) AS tx_count
		FROM relevant;
	`, address).Scan(&balance, &count)

	if err != nil {
		log.Printf("Error getting balance for address %s: %v", address, err)
		return GetBalanceResult{}, err
	}

	return GetBalanceResult{
		Address:      address,
		Balance:      balance,
		Transactions: count,
	}, nil
}

type GetTransactionsAndFeesResult struct {
	Transactions []Transaction `json:"transactions"`
	Fees         []Fee         `json:"fees"`
}

func (db *DBClient) GetTransactionsAndFees(ctx context.Context, address string) (GetTransactionsAndFeesResult, error) {
	// NOTE  In the name of doing this in one query, error prone, nil dereference could happen
	const query = `
		SELECT 
			'tx' as type,
			hash,
			block_index,
			from_address,
			to_address,
			value,
			type as tx_type,
			succesful,
			timestamp AT TIME ZONE 'UTC' as timestamp,
			NULL as fee_amount
		FROM transactions
		WHERE from_address = $1 OR to_address = $1
		UNION ALL
		SELECT 
			'fee' as type,
			transaction_hash as hash,
			NULL,
			from_address,
			NULL,
			NULL,
			NULL,
			NULL,
			NULL,
			amount as fee_amount
		FROM fees
		WHERE from_address = $1
		ORDER BY timestamp NULLS LAST
	`

	rows, err := db.Conn.Query(ctx, query, address)
	if err != nil {
		return GetTransactionsAndFeesResult{}, err
	}
	defer rows.Close()

	var result GetTransactionsAndFeesResult

	for rows.Next() {
		var (
			typ          string
			hash         string
			blockIndex   *string
			from         string
			to           *string
			valueStr     *string
			txType       *string
			succesful    *bool
			timestamp    *time.Time
			feeAmountStr *string
		)

		if err := rows.Scan(&typ, &hash, &blockIndex, &from, &to, &valueStr, &txType, &succesful, &timestamp, &feeAmountStr); err != nil {
			return GetTransactionsAndFeesResult{}, err
		}

		switch typ {
		case "tx":
			val, err := decimal.NewFromString(*valueStr)
			if err != nil {
				log.Printf("Error parsing value %s for transaction %s: %v", *valueStr, hash, err)
				return GetTransactionsAndFeesResult{}, err
			}

			result.Transactions = append(result.Transactions, Transaction{
				Hash:       hash,
				Type:       *txType,
				Value:      val,
				From:       from,
				To:         *to,
				BlockIndex: *blockIndex,
				Succesful:  *succesful,
				Timestamp:  *timestamp,
			})

		case "fee":
			feeAmount, err := decimal.NewFromString(*feeAmountStr)
			if err != nil {
				log.Printf("Error parsing fee amount %s for transaction %s: %v", *feeAmountStr, hash, err)
				return GetTransactionsAndFeesResult{}, err
			}

			result.Fees = append(result.Fees, Fee{
				TransactionHash: hash,
				FromAddress:     from,
				Amount:          feeAmount,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return GetTransactionsAndFeesResult{}, err
	}

	return result, nil
}
