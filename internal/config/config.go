package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	// Addresses I will index
	Addresses []string `env:"ADDRESSES,required"`
	// Block to prefetch, then use the latest one as the startIdx
	Blocks   []uint64 `env:"BLOCKS,required"`
	Database DBConfig
	BaseAPI  BaseAPIConfig
	Server   ServerConfig
}

type DBConfig struct {
	Username string `env:"DB_USERNAME,required"`
	Password string `env:"DB_PASSWORD,required"`
	Name     string `env:"DB_NAME,required"`
	Host     string `env:"DB_HOST,default=localhost"`
	Port     uint16 `env:"DB_PORT,default=5432"`
}

type ServerConfig struct {
	Port uint16 `env:"API_PORT,default=3000"`
}

func (dbc DBConfig) String() string {
	return fmt.Sprintf("postgresql://%s:%s@%s/%s?connect_timeout=5", dbc.Username, dbc.Password, dbc.Host, dbc.Name)
}

type BaseAPIConfig struct {
	BaseURL      string `env:"BASE_API_BASE_URL,default=https://base-rpc.publicnode.com"`
	BaseDebugURL string `env:"BASE_API_BASE_DEBUG_URL,default=https://docs-demo.base-mainnet.quiknode.pro"`
}

func New(ctx context.Context) (*Config, error) {
	var cfg Config
	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}
	return &cfg, nil
}
