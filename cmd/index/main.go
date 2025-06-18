package main

import (
	"context"
	"log"

	"github.com/danilevy1212/baseidx-wt/internal/config"
	"github.com/danilevy1212/baseidx-wt/internal/data"
	"github.com/danilevy1212/baseidx-wt/internal/database"
	"github.com/danilevy1212/baseidx-wt/internal/rpc"
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

	rpcClient := rpc.NewClient(cfg.BaseAPI.BaseURL)

	deleteMeJustATestOfRPCClient(rpcClient)
}

func deleteMeJustATestOfRPCClient(c rpc.Client) {
	latestBlock, err := c.GetLastestBlock()

	if err != nil {
		log.Fatal("Error getting latest block:", err)
	}
	log.Printf("Latest block: %+v", latestBlock)

	// Example of getting a specific block by number
	blockNumber, err := data.NewHexFromString(latestBlock.Result)
	if err != nil {
		log.Fatal("Error converting latest block number:", err)
	}

	block, err := c.GetBlockByNumber(*blockNumber, true)
	if err != nil {
		log.Fatal("Error getting block by number:", err)
	}
	log.Printf("Block %d: %+v", blockNumber, block)

	// Example of getting balance for an address
	balance, err := c.GetBalance("0xEED8504ee6563c51a64e5306115fCB3Ceb59bC71")
	if err != nil {
		log.Fatal("Error getting balance:", err)
	}
	log.Printf("Balance for address: %s is %+v", "0xEED8504ee6563c51a64e5306115fCB3Ceb59bC71", balance)

	// Example of getting receipts
	receipts, err := c.GetBlockReceipts(*blockNumber)
	if err != nil {
		log.Fatal("Error getting receipts by block number:", err)
	}
	log.Printf("Receipts for block %d: %+v", blockNumber, receipts)
}
