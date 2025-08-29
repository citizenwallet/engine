package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/internal/ethrequest"
	"github.com/citizenwallet/engine/pkg/engine"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/fiatjaf/eventstore/postgresql"
	"github.com/nbd-wtf/go-nostr"
)

// getERC20Symbol calls the symbol() method on an ERC20 contract
func getERC20Symbol(evm *ethrequest.EthService, contractAddress common.Address) (string, error) {
	// ERC20 symbol() function selector: 0x95d89b41
	symbolSelector := common.Hex2Bytes("95d89b41")

	result, err := evm.CallContract(ethereum.CallMsg{
		To:   &contractAddress,
		Data: symbolSelector,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to call symbol(): %w", err)
	}

	// Decode the result using ABI
	if len(result) == 0 {
		return "", fmt.Errorf("empty result from symbol() call")
	}

	// Create a simple ABI for the symbol() function
	erc20ABI, err := abi.JSON(strings.NewReader(`[{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"type":"function"}]`))
	if err != nil {
		return "", fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Decode the result
	var symbol string
	err = erc20ABI.UnpackIntoInterface(&symbol, "symbol", result)
	if err != nil {
		return "", fmt.Errorf("failed to unpack symbol result: %w", err)
	}

	return symbol, nil
}

// getERC20Decimals calls the decimals() method on an ERC20 contract
func getERC20Decimals(evm *ethrequest.EthService, contractAddress common.Address) (uint8, error) {
	// ERC20 decimals() function selector: 0x313ce567
	decimalsSelector := common.Hex2Bytes("313ce567")

	result, err := evm.CallContract(ethereum.CallMsg{
		To:   &contractAddress,
		Data: decimalsSelector,
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to call decimals(): %w", err)
	}

	// Decode the result using ABI
	if len(result) == 0 {
		return 0, fmt.Errorf("empty result from decimals() call")
	}

	// Create a simple ABI for the decimals() function
	erc20ABI, err := abi.JSON(strings.NewReader(`[{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"type":"function"}]`))
	if err != nil {
		return 0, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Decode the result
	var decimals uint8
	err = erc20ABI.UnpackIntoInterface(&decimals, "decimals", result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack decimals result: %w", err)
	}

	return decimals, nil
}

func MigrateLogs(ctx context.Context, evm *ethrequest.EthService, secretKey, pubkey string, db *db.DB, ndb *postgresql.PostgresBackend) error {
	events, err := db.EventDB.GetEvents()
	if err != nil {
		return err
	}

	maxDate := time.Now()
	maxDate.AddDate(0, 0, 1)

	for _, event := range events {
		log.Printf("Migrating logs for event: %s", event.Name)
		topic := event.GetTopic0FromEventSignature()

		contract := common.HexToAddress(event.Contract)

		symbol, err := getERC20Symbol(evm, contract)
		if err != nil {
			log.Printf("Error getting ERC20 symbol: %v", err)
			return err
		}

		decimals, err := getERC20Decimals(evm, contract)
		if err != nil {
			log.Printf("Error getting ERC20 decimals: %v", err)
			return err
		}

		offset := 0
		for {
			logs, err := db.LogDB.GetAllPaginatedLogs(event.Contract, topic.Hex(), maxDate, 100, offset)
			if err != nil {
				return err
			}

			if len(logs) == 0 {
				break
			}

			for _, log := range logs {
				ev := convertLogToEvent(pubkey, log, symbol, decimals)

				err = ev.Sign(secretKey)
				if err != nil {
					return err
				}

				err = ndb.SaveEvent(ctx, ev)
				if err != nil {
					return err
				}
			}

			if len(logs) == 0 {
				break
			}

			offset += len(logs)
			log.Printf("Migrated %d logs", offset)
		}

	}
	return nil
}

func convertLogToEvent(pubkey string, log *engine.Log, symbol string, decimals uint8) *nostr.Event {

	transferData := engine.LogTransferData{}
	b, err := json.Marshal(*log.Data)
	if err != nil {
		return nil
	}

	fmt.Println("Log data:", string(b))

	err = json.Unmarshal(b, &transferData)
	if err != nil {
		fmt.Println("Error unmarshalling log data:", err)
		transferData.From = log.Sender
		transferData.To = log.To
		transferData.Value = "0"
	}

	// Convert string value to float64 for calculation
	valueFloat, _ := strconv.ParseFloat(transferData.Value, 64)
	formattedValue := fmt.Sprintf("%.2f", valueFloat/math.Pow10(int(decimals)))

	return &nostr.Event{
		PubKey:    pubkey,
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      1,
		Content:   fmt.Sprintf("%s %s sent from %s to %s \n\n%s", formattedValue, symbol, transferData.From, transferData.To, log.TxHash),
		Tags:      []nostr.Tag{},
	}
}
