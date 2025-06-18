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
	defer dbClient.Close(ctx)

	if err := dbClient.CreateSchema(ctx); err != nil {
		log.Fatal("Error creating schema: ", err)
	}

	log.Println("Database schema created successfully")
}
