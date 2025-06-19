package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/danilevy1212/baseidx-wt/internal/config"
	"github.com/danilevy1212/baseidx-wt/internal/database"

	"github.com/gin-gonic/gin"
)

func main() {
	ctx := context.Background()
	cfg, err := config.New(ctx)
	if err != nil {
		log.Fatalf("Error parsing config: %v", err)
	}

	db, err := database.New(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("Error creating database client: %v", err)
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
		})
	})

	r.GET("/accounts/:account/balance", func(c *gin.Context) {
		account := c.Param("account")
		if account == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "account parameter is required"})
			return
		}

		balance, err := db.GetBalance(ctx, account)
		if err != nil {
			log.Printf("Error getting balance for account %s: %v", account, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		if balance.Transactions <= 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "account not found or no transactions"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"account": balance.Address,
			"balance": balance.Balance,
		})
	})

	r.GET("/accounts/:account/transactions", func(c *gin.Context) {
		account := c.Param("account")
		if account == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "account parameter is required"})
			return
		}

		result, err := db.GetTransactionsAndFees(ctx, account)
		if err != nil {
			log.Printf("Error getting transactions and fees for account %s: %v", account, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, result)
	})

	r.GET("/transactions", func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service unavailable"})
	})

	if err := r.Run(fmt.Sprintf(":%d", cfg.Server.Port)); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
