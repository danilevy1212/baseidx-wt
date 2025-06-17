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
