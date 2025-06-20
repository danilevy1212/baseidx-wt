package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

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
		account := strings.ToLower(c.Param("account"))
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
			"account": account,
			"balance": balance.Balance,
		})
	})

	r.GET("/accounts/:account/transactions", func(c *gin.Context) {
		account := strings.ToLower(c.Param("account"))
		if account == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "account parameter is required"})
			return
		}

		result, err := db.GetTransactionsFromAddress(ctx, account)
		if err != nil {
			log.Printf("Error getting transactions and fees for account %s: %v", account, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, result)
	})

	r.GET("/transactions", func(c *gin.Context) {
		startStr := c.Query("start")
		endStr := c.Query("end")

		if startStr == "" || endStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "start and end parameters are required"})
			return
		}

		start, err := parseTime(startStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid start: %v", err)})
			return
		}
		end, err := parseTime(endStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid end: %v", err)})
			return
		}

		if start.After(end) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "start time must be before end time"})
			return
		}

		transactions, err := db.GetTransactionsInRange(ctx, start, end)
		if err != nil {
			log.Printf("Error getting transactions in range %s to %s: %v", startStr, endStr, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"start":        start.Format(time.RFC3339),
			"end":          end.Format(time.RFC3339),
			"transactions": transactions,
		})
	})

	if err := r.Run(fmt.Sprintf(":%d", cfg.Server.Port)); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func parseTime(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid RFC3339 timestamp (must be YYYY-MM-DDTHH:MM:SSZ): %w", err)
	}
	if t.Location() != time.UTC {
		return time.Time{}, fmt.Errorf("timestamp must be in UTC and end with 'Z'")
	}
	return t, nil
}
