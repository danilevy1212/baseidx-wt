package database

import (
	"time"

	"github.com/shopspring/decimal"
)

type Transaction struct {
	Hash       string          `db:"hash" json:"hash"` // Primary key
	Type       string          `db:"type" json:"type"` // "transfer", "call" or "fee", with more time I would make it an enum type
	Value      decimal.Decimal `db:"value" json:"value"`
	From       string          `db:"from_address" json:"from"`
	To         string          `db:"to_address" json:"to"`
	BlockIndex string          `db:"block_index" json:"blockIndex"`
	Succesful  bool            `db:"succesful" json:"susccesful"`
	Timestamp  time.Time       `db:"timestamp" json:"timestamp"` // For range queries
}
