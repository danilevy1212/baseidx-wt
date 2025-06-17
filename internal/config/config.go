package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	// Addresses I will index
	Addresses  []string `env:"ADDRESSES,required"`
	StartBlock uint64   `env:"START_BLOCK,required"`
	Database   DBConfig
}

type DBConfig struct {
	Username string `env:"DB_USERNAME,required"`
	Password string `env:"DB_PASSWORD,required"`
	Name     string `env:"DB_NAME,required"`
	Host     string `env:"DB_HOST,default=localhost"`
	Port     uint16 `env:"DB_PORT,default=5432"`
}

func (dbc DBConfig) String() string {
	return fmt.Sprintf("postgresql://%s:%s@%s/%s?connect_timeout=5", dbc.Username, dbc.Password, dbc.Host, dbc.Name)
}

func New(ctx context.Context) (*Config, error) {
	var cfg Config
	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}
	return &cfg, nil
}
