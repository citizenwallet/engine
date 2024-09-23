package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/citizenwallet/engine/internal/config"
	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/internal/ethrequest"
	"github.com/citizenwallet/engine/pkg/engine"
	_ "github.com/mattn/go-sqlite3"
)

const (
	contractAddress = "0x5815E61eF72c9E6107b5c5A05FD121F334f7a7f1"
	transferTopic   = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	batchSize       = 1000
	migrationSuffix = "migration"
)

func main() {
	// Parse command-line flags
	env := flag.String("env", ".env", "path to .env file")
	flag.Parse()

	// Load configuration from .env file
	ctx := context.Background()
	conf, err := config.New(ctx, *env)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Open SQLite database
	sqliteDBPath := conf.DBName // Assuming DBName contains the SQLite file path
	sqliteDB, err := sql.Open("sqlite3", sqliteDBPath)
	if err != nil {
		log.Fatalf("Error opening SQLite database: %v", err)
	}
	defer sqliteDB.Close()

	// Construct PostgreSQL connection string
	evm, err := ethrequest.NewEthService(ctx, conf.RPCURL)
	if err != nil {
		log.Fatal(err)
	}

	chid, err := evm.ChainID()
	if err != nil {
		log.Fatal(err)
	}

	d, err := db.NewDB(chid, conf.DBSecret, conf.DBUser, conf.DBPassword, conf.DBName, conf.DBHost, conf.DBReaderHost)
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	// Perform migration
	err = migrateData(sqliteDB, d.LogDB)
	if err != nil {
		log.Fatalf("Error during migration: %v", err)
	}

	log.Println("Migration completed successfully")
}

func migrateData(sqliteDB *sql.DB, logDB *db.LogDB) error {
	offset := 0
	for {
		transfers, err := getTransfers(sqliteDB, offset, batchSize)
		if err != nil {
			return fmt.Errorf("error getting transfers: %v", err)
		}

		if len(transfers) == 0 {
			break
		}

		logs := convertTransfersToLogs(transfers)

		err = logDB.AddLogs(logs)
		if err != nil {
			return fmt.Errorf("error adding logs: %v", err)
		}

		offset += len(transfers)
		log.Printf("Migrated %d transfers", offset)
	}

	return nil
}

func getTransfers(db *sql.DB, offset, limit int) ([]*Transfer, error) {
	query := fmt.Sprintf(`
		SELECT hash, tx_hash, token_id, created_at, from_addr, to_addr, nonce, value, data, status
		FROM t_transfers_%s
		ORDER BY created_at
		LIMIT ? OFFSET ?
	`, os.Getenv("CHAIN_ID"))

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transfers []*Transfer
	for rows.Next() {
		var t Transfer
		var valueStr string
		err := rows.Scan(&t.Hash, &t.TxHash, &t.TokenID, &t.CreatedAt, &t.From, &t.To, &t.Nonce, &valueStr, &t.Data, &t.Status)
		if err != nil {
			return nil, err
		}
		t.Value = new(big.Int)
		t.Value.SetString(valueStr, 10)
		transfers = append(transfers, &t)
	}

	return transfers, nil
}

func convertTransfersToLogs(transfers []*Transfer) []*engine.Log {
	var logs []*engine.Log
	for _, t := range transfers {
		data := map[string]interface{}{
			"topic": transferTopic,
			"from":  t.From,
			"to":    t.To,
			"value": t.Value.String(),
		}
		dataJSON, _ := json.Marshal(data)
		dataRaw := json.RawMessage(dataJSON)

		b, err := json.Marshal(t.Data)
		if err != nil {
			log.Fatalf("Error marshalling data: %v", err)
		}

		var extraDataRaw json.RawMessage
		if t.Data != nil {
			extraDataRaw = json.RawMessage(b)
		}

		log := &engine.Log{
			Hash:      t.Hash,
			TxHash:    t.TxHash,
			CreatedAt: t.CreatedAt,
			UpdatedAt: time.Now(),
			Nonce:     0,
			Sender:    t.From,
			To:        contractAddress,
			Value:     big.NewInt(0),
			Data:      &dataRaw,
			ExtraData: &extraDataRaw,
			Status:    engine.LogStatusSuccess,
		}
		logs = append(logs, log)
	}
	return logs
}
