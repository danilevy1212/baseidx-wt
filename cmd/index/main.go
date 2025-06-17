package main

import (
	"context"
	"log"

	"github.com/danilevy1212/baseidx-wt/internal/config"
	"github.com/danilevy1212/baseidx-wt/internal/database"
)

func main() {
	ctx := context.Background()
	cfg, err := config.New(ctx)

	if err != nil {
		log.Fatal("Error parsing config", err)
	}

	log.Printf("Config: %+v", cfg)

	dbClient, err := database.New(ctx, cfg.Database)
	if err != nil {
		log.Fatal("Error creating database client", err)
	}

	if err = dbClient.Ping(ctx); err != nil {
		log.Fatal("Error pinging database", err)
	}

	log.Println("Database connection successful")
}
