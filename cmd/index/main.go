package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/danilevy1212/baseidx-wt/internal/config"
	"github.com/danilevy1212/baseidx-wt/internal/data"
	"github.com/danilevy1212/baseidx-wt/internal/database"
	"github.com/danilevy1212/baseidx-wt/internal/rpc"
)

var rpcClient rpc.Client
var dbClient *database.DBClient

func main() {
	ctx := context.Background()
	cfg, err := config.New(ctx)

	if err != nil {
		log.Fatal("Error parsing config", err)
	}
	log.Printf("Config: %+v", cfg)

	dbClient, err = database.New(ctx, cfg.Database)
	if err != nil {
		log.Fatal("Error creating database client", err)
	}

	if err = dbClient.Ping(ctx); err != nil {
		log.Fatal("Error pinging database", err)
	}

	log.Println("Database connection successful")

	rpcClient = rpc.NewClient(cfg.BaseAPI.BaseURL, cfg.BaseAPI.BaseDebugURL)

	lastBlock, err := rpcClient.GetLastestBlock()
	if err != nil {
		log.Fatal("Error getting latest block", err)
	}
	log.Printf("last block: %v", lastBlock.Result)

	lastBlockIdx, err := data.NewHexFromString(lastBlock.Result)
	if err != nil {
		log.Fatal("Error parsing last block index", err)
	}

	cfg.Blocks = deduplicate(cfg.Blocks)
	slices.Sort(cfg.Blocks)

	accounts := map[string]bool{}
	for _, addr := range cfg.Addresses {
		accounts[strings.ToLower(addr)] = true
	}

	log.Printf("Accounts to index: %v", accounts)

	for _, blockIdx := range cfg.Blocks {
		if blockIdx > lastBlockIdx.Uint64() {
			log.Printf("Skipping block %d, it is greater than the latest block %d", blockIdx, lastBlockIdx.Uint64())
			break
		}

		transactions, err := processBlock(*data.NewHexFromUint64(blockIdx), accounts)
		if err != nil {
			log.Printf("Error processing block %d: %v", blockIdx, err)
			continue
		}

		// Bulk update transactions
		log.Printf("Processed block %d with %d transactions", blockIdx, len(transactions))

		if err := dbClient.UpsertTransactions(ctx, transactions); err != nil {
			log.Printf("Error upserting transactions for block %d: %v", blockIdx, err)
			continue
		}
	}
}

// TODO  Bring this to it's own service later, so I can re-use it in the API
func processBlock(blockIdx data.Hex, accounts map[string]bool) ([]database.Transaction, error) {
	blockDTO, err := rpcClient.GetBlockByNumber(blockIdx, true)
	if err != nil {
		log.Printf("Error getting block %s: %v", blockIdx.String(), err)
		return nil, err
	}
	blockTimestampHex, err := data.NewHexFromString(blockDTO.Result.Timestamp)
	if err != nil {
		log.Printf("Error parsing block timestamp %s: %v", blockDTO.Result.Timestamp, err)
		return nil, err
	}

	blockTimestamp := time.Unix(blockTimestampHex.Int64(), 0)

	var receiptsDTO *rpc.BlockReceiptsDTO
	transactions := []database.Transaction{}

	log.Printf("Processing block %s at index %d, at timestamp %s", blockDTO.Result.Number, blockIdx.Uint64(), blockDTO.Result.Timestamp)

	// Go through the transactions
	for _, txDto := range blockDTO.Result.Transactions {
		// Irrelevant transaction
		if !accounts[txDto.From] && !accounts[txDto.To] {
			continue
		}

		var receiptDTO *rpc.Receipt
		// Get the receipts if we haven't already
		if receiptsDTO == nil {
			receiptsDTO, err = rpcClient.GetBlockReceipts(blockIdx)

			if err != nil {
				log.Printf("Error getting receipts for block %s: %v", blockIdx.String(), err)
				continue
			}
		}

		// Get the receipt for the matching TX
		for _, r := range receiptsDTO.Result {
			if r.TransactionHash == txDto.Hash {
				receiptDTO = &r
				break
			}
		}

		if receiptDTO == nil {
			log.Printf("No receipt found for transaction %s in block %s", txDto.Hash, blockIdx.String())
			continue
		}

		log.Printf("Processing transaction %s from %s to %s with value %s at block index %s", txDto.Hash, txDto.From, txDto.To, txDto.Value, blockIdx.String())

		var trx database.Transaction

		trx.Timestamp = blockTimestamp
		trx.BlockIndex = blockDTO.Result.Number

		trx.Hash = txDto.Hash
		trx.To = txDto.To
		trx.From = txDto.From

		trx.Type = "transfer"
		if txDto.Input != "0x" {
			trx.Type = "call"
		}

		if receiptDTO.Status == "0x1" {
			trx.Succesful = true
		}

		amount, err := data.NewHexFromString(txDto.Value)
		if err != nil {
			log.Printf("Error parsing transaction value %s: %v", txDto.Value, err)
			continue
		}

		trx.Value = decimal.NewFromBigInt(amount.Int, 0)

		log.Printf("Transaction details: %+v", trx)

		transactions = append(transactions, trx)

		// Recursively add calls
		if trx.Type == "call" {
			err := processContractCall(trx, accounts, &transactions)
			if err != nil {
				log.Printf("Error processing contract call for transaction %s: %v", trx.Hash, err)
				continue
			}
		}

		// Fee
		var fee database.Transaction
		fee.Hash = trx.Hash + "_fee"
		fee.Type = "fee"
		fee.From = trx.From
		fee.To = trx.From // This is not really used, but it is a valid address
		fee.BlockIndex = trx.BlockIndex
		fee.Timestamp = trx.Timestamp
		fee.Succesful = true // Fees are always successful

		l1Fee := decimal.Zero
		if receiptDTO.L1Fee != nil {
			l1FeeHex, err := data.NewHexFromString(*receiptDTO.L1Fee)
			if err != nil {
				log.Printf("Error parsing L1 fee %s: %v", *receiptDTO.L1Fee, err)
				continue
			}
			l1Fee = decimal.NewFromBigInt(l1FeeHex.Int, 0)
		}
		effectiveGasPriceHex, err := data.NewHexFromString(receiptDTO.EffectiveGasPrice)
		if err != nil {
			log.Printf("Error parsing effective gas price %s: %v", receiptDTO.EffectiveGasPrice, err)
			continue
		}
		effectiveGasPrice := decimal.NewFromBigInt(effectiveGasPriceHex.Int, 0)

		gasUsedHex, err := data.NewHexFromString(receiptDTO.GasUsed)
		if err != nil {
			log.Printf("Error parsing gas used %s: %v", receiptDTO.GasUsed, err)
			continue
		}
		gasUsed := decimal.NewFromBigInt(gasUsedHex.Int, 0)

		fee.Value = effectiveGasPrice.Mul(gasUsed).Add(l1Fee)

		log.Printf("Fee details: %+v", fee)

		transactions = append(transactions, fee)
	}

	return transactions, nil
}

func processContractCall(origin database.Transaction, accounts map[string]bool, transactions *[]database.Transaction) error {
	calls, err := rpcClient.GetTransactionCallTrace(origin.Hash)

	if err != nil {
		log.Printf("Error getting call trace for transaction %s: %v", origin.Hash, err)
		return err
	}

	return recurseCallStack(origin, calls.Result.Calls, accounts, transactions, new(int))
}

func recurseCallStack(origin database.Transaction, callStack []rpc.CallTrace, accounts map[string]bool, transactions *[]database.Transaction, count *int) error {
	for _, call := range callStack {
		// Skip if no value was transferred
		if call.Value == "0x0" || call.Value == "0x" || call.Value == "" {
			// Still recurse to deeper calls even if this call itself had no value
			if len(call.Calls) > 0 {
				err := recurseCallStack(origin, call.Calls, accounts, transactions, count)
				if err != nil {
					return err
				}
			}
			continue
		}

		// Parse value
		valHex, err := data.NewHexFromString(call.Value)
		if err != nil {
			log.Printf("Error parsing internal call value %s: %v", call.Value, err)
			continue
		}

		// Only include if from or to is in accounts map
		if !accounts[call.From] && !accounts[call.To] {
			// Still recurse
			if len(call.Calls) > 0 {
				err := recurseCallStack(origin, call.Calls, accounts, transactions, count)
				if err != nil {
					return err
				}
			}
			continue
		}

		// Increment the count for unique internal calls
		*count++

		trx := database.Transaction{
			From:       call.From,
			To:         call.To,
			Hash:       origin.Hash + "_internal_" + fmt.Sprintf("%d", *count),
			Value:      decimal.NewFromBigInt(valHex.Int, 0),
			BlockIndex: origin.BlockIndex,
			Timestamp:  origin.Timestamp,
			Succesful:  true,
		}

		trx.Type = "transfer"
		if call.Input != "0x" {
			trx.Type = "call"
		}

		log.Printf("Processing internal call %d: %+v", *count, trx)

		*transactions = append(*transactions, trx)

		// Recurse
		if len(call.Calls) > 0 {
			err := recurseCallStack(origin, call.Calls, accounts, transactions, count)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// TODO  Same, aka, move to utils or something
func deduplicate[T comparable](original []T) []T {
	unique := make([]T, 0, len(original))
	seen := make(map[T]struct{})

	for _, item := range original {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			unique = append(unique, item)
		}
	}

	return unique
}

// TODO  This could be interesting, but only if I have time for a "range sweep" endpoint
// func scrapBlockChain(startIdx, endIdx data.Hex) error {
// 	currentIdx := startIdx
//
// 	log.Printf("Starting to scrap blockchain from %d to %d", startIdx.Uint64(), endIdx.Uint64())
// 	totalBlocks := new(big.Int).Sub(endIdx.Int, startIdx.Int)
//
// 	log.Printf("Total blocks to process: %d", totalBlocks.Uint64())
//
// 	for currentIdx.Uint64() != endIdx.Uint64() {
// 		if err := processBlock(currentIdx); err != nil {
// 			return err
// 		}
// 	}
//
// 	return nil
// }
