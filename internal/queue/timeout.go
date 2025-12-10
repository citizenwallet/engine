package queue

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/pkg/engine"
)

// TimeoutService checks for timeout userops and marks them as reverted if their tx receipts are not found on chain
type TimeoutService struct {
	db  *db.DB
	evm engine.EVMRequester
}

// NewTimeoutService creates a new TimeoutService
func NewTimeoutService(db *db.DB, evm engine.EVMRequester) *TimeoutService {
	return &TimeoutService{
		db:  db,
		evm: evm,
	}
}

// Start runs the timeout checker every minute
func (s *TimeoutService) Start(ctx context.Context) error {
	log.Default().Println("starting timeout checker service...")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Run once immediately on startup
	s.checkTimeoutUserOps()

	for {
		select {
		case <-ticker.C:
			s.checkTimeoutUserOps()
		case <-ctx.Done():
			log.Default().Println("stopping timeout checker service...")
			return ctx.Err()
		}
	}
}

// checkTimeoutUserOps fetches timeout userops older than 2 minutes and checks their receipts
func (s *TimeoutService) checkTimeoutUserOps() {
	ops, err := s.db.UserOpDB.GetTimeoutUserOpsOlderThan(2)
	if err != nil {
		log.Default().Println("error fetching timeout userops:", err.Error())
		return
	}

	if len(ops) == 0 {
		return
	}

	log.Default().Printf("checking %d timeout userops for receipts...\n", len(ops))

	for _, op := range ops {
		// If there's no tx_hash, the transaction was never sent - mark as reverted
		if op.TxHash == nil || *op.TxHash == "" {
			err := s.db.UserOpDB.UpdateStatus(op.UserOpHash, db.UserOpStatusReverted)
			if err != nil {
				log.Default().Printf("error updating userop %s to reverted: %s\n", op.UserOpHash, err.Error())
			} else {
				log.Default().Printf("marked userop %s as reverted (no tx_hash)\n", op.UserOpHash)
			}
			continue
		}

		// Check for receipt on chain
		hasReceipt := s.checkReceipt(*op.TxHash)
		if !hasReceipt {
			err := s.db.UserOpDB.UpdateStatus(op.UserOpHash, db.UserOpStatusReverted)
			if err != nil {
				log.Default().Printf("error updating userop %s to reverted: %s\n", op.UserOpHash, err.Error())
			} else {
				log.Default().Printf("marked userop %s as reverted (no receipt for tx %s)\n", op.UserOpHash, *op.TxHash)
			}
		} else {
			// Receipt found - mark as success since the tx was mined
			err := s.db.UserOpDB.UpdateStatusToSuccess(op.UserOpHash)
			if err != nil {
				log.Default().Printf("error updating userop %s to success: %s\n", op.UserOpHash, err.Error())
			} else {
				log.Default().Printf("marked userop %s as success (receipt found for tx %s)\n", op.UserOpHash, *op.TxHash)
			}
		}
	}
}

// checkReceipt checks if a transaction receipt exists on chain
func (s *TimeoutService) checkReceipt(txHash string) bool {
	params, err := json.Marshal([]string{txHash})
	if err != nil {
		log.Default().Println("error marshaling tx hash:", err.Error())
		return false
	}

	var result any
	err = s.evm.Call("eth_getTransactionReceipt", &result, params)
	if err != nil {
		log.Default().Printf("error getting receipt for tx %s: %s\n", txHash, err.Error())
		return false
	}

	// If result is nil, no receipt was found
	return result != nil
}
