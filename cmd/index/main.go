package main

import (
	"context"
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

	rpcClient = rpc.NewClient(cfg.BaseAPI.BaseURL)

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

		if err := processBlock(*data.NewHexFromUint64(blockIdx), accounts); err != nil {
			log.Printf("Error processing block %d: %v", blockIdx, err)
			continue
		}
	}
}

// TODO  Bring this to it's own service later, so I can re-use it in the API
func processBlock(blockIdx data.Hex, accounts map[string]bool) error {
	blockDTO, err := rpcClient.GetBlockByNumber(blockIdx, true)
	if err != nil {
		log.Printf("Error getting block %s: %v", blockIdx.String(), err)
		return err
	}
	blockTimestampHex, err := data.NewHexFromString(blockDTO.Result.Timestamp)
	if err != nil {
		log.Printf("Error parsing block timestamp %s: %v", blockDTO.Result.Timestamp, err)
		return err
	}

	blockTimestamp := time.Unix(blockTimestampHex.Int64(), 0)

	var receiptsDTO *rpc.BlockReceiptsDTO

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

		// Outgoing
		if origin := strings.ToLower(txDto.From); accounts[origin] {
			trx.Account = origin
		}

		// Incoming
		if receiver := strings.ToLower(txDto.To); accounts[receiver] {
			trx.Account = receiver
		}

		log.Printf("Transaction details: %+v", trx)

		if err := dbClient.UpsertTransaction(context.Background(), trx); err != nil {
			log.Printf("Error inserting transaction %s: %v", trx.Hash, err)
			continue
		}

		// Fee
		var fee database.Fee
		fee.TransactionHash = trx.Hash
		fee.FromAddress = trx.From

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

		fee.Amount = effectiveGasPrice.Mul(gasUsed).Add(l1Fee)

		if err := dbClient.UpsertFee(context.Background(), fee); err != nil {
			log.Printf("Error inserting fee for transaction %s: %v", trx.Hash, err)
			continue
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
