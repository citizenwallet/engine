package queue

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/internal/ws"
	comm "github.com/citizenwallet/engine/pkg/common"
	"github.com/citizenwallet/engine/pkg/engine"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/tokenEntryPoint"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

type UserOpService struct {
	inProgress map[common.Address][]string
	mu         sync.Mutex
	db         *db.DB
	evm        engine.EVMRequester
	pushq      *Service
	pools      *ws.ConnectionPools
}

func NewUserOpService(db *db.DB,
	evm engine.EVMRequester,
	pushq *Service,
	pools *ws.ConnectionPools) *UserOpService {
	return &UserOpService{
		inProgress: map[common.Address][]string{},
		db:         db,
		evm:        evm,
		pushq:      pushq,
		pools:      pools,
	}
}

// Process method processes messages of type []engine.Message and returns processed messages and an errors if any.
func (s *UserOpService) Process(messages []engine.Message) (invalid []engine.Message, errors []error) {
	invalid = []engine.Message{}
	errors = []error{}

	messagesByEntryPoint := map[common.Address][]engine.Message{}
	txmByEntryPoint := map[common.Address][]engine.UserOpMessage{}

	// first organize messages by txm.EntryPoint
	for _, message := range messages {
		// Type assertion to check if the msgs... is of type engine.UserOpMessage
		txm, ok := message.Message.(engine.UserOpMessage)
		if !ok {
			// If the message is not of type engine.UserOpMessage, return an error
			invalid = append(invalid, message)
			errors = append(errors, fmt.Errorf("invalid tx msgs..."))
			continue
		}

		messagesByEntryPoint[txm.EntryPoint] = append(messagesByEntryPoint[txm.EntryPoint], message)
		txmByEntryPoint[txm.EntryPoint] = append(txmByEntryPoint[txm.EntryPoint], txm)
	}

	// go through each entry point and process the messages
	for entrypoint, txms := range txmByEntryPoint {
		sampleTxm := txms[0] // use the first txm to get information we need to process the messages
		msgs := messagesByEntryPoint[entrypoint]

		// Fetch the sponsor's corresponding private key from the database
		sponsorKey, err := s.db.SponsorDB.GetSponsor(sampleTxm.Paymaster.Hex())
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				for range msgs {
					errors = append(errors, err)
				}
			}
			continue
		}

		// Generate ecdsa.PrivateKey from bytes
		privateKey, err := comm.HexToPrivateKey(sponsorKey.PrivateKey)
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		// Get the public key from the private key
		publicKey := privateKey.Public().(*ecdsa.PublicKey)

		// Convert the public key to an Ethereum address
		sponsor := crypto.PubkeyToAddress(*publicKey)

		// Get the nonce for the sponsor's address
		nonce, err := s.evm.NonceAt(context.Background(), sponsor, nil)
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		// Get the in progress transactions for the entrypoint and increment the nonce
		inProgress := s.inProgress[entrypoint]
		nonce += uint64(len(inProgress))

		// Parse the contract ABI
		parsedABI, err := tokenEntryPoint.TokenEntryPointMetaData.GetAbi()
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		ops := []tokenEntryPoint.UserOperation{}

		for _, txm := range txms {
			ops = append(ops, tokenEntryPoint.UserOperation(txm.UserOp))
		}

		// Pack the function name and arguments into calldata
		data, err := parsedABI.Pack("handleOps", ops, sampleTxm.EntryPoint)
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		// Create a new transaction
		tx, err := s.evm.NewTx(nonce, sponsor, sampleTxm.EntryPoint, data, false)
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		// Sign the transaction
		signedTx, err := types.SignTx(tx, types.NewLondonSigner(sampleTxm.ChainId), privateKey)
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		signedTxHash := signedTx.Hash().Hex()

		// update inProgress
		s.mu.Lock()
		s.inProgress[entrypoint] = append(s.inProgress[entrypoint], signedTxHash)
		s.mu.Unlock()

		insertedLogs := map[common.Address][]*engine.Log{}

		ldb := s.db.LogDB
		edb := s.db.EventDB

		events, err := edb.GetEvents()
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		for _, txm := range txms {
			// Detect if this user operation is a transfer using the call data

			userop := txm.UserOp
			data, ok := txm.Data.(*json.RawMessage)
			if !ok {
				data = nil
			}

			if data == nil {
				// if there is no data, it is impossible for us to generate a stable unique hash
				// so we skip it
				continue
			}

			var dataMap map[string]any
			if err := json.Unmarshal(*data, &dataMap); err != nil {
				continue
			}

			// there is data, let's check if it is valid according to any of the event signatures that we are indexing
			valid := false
			for _, event := range events {
				if event.IsValidData(dataMap) {
					// we have a match
					valid = true
					break
				}
			}

			if !valid {
				continue
			}

			txdata, ok := txm.ExtraData.(*json.RawMessage)
			if !ok {
				// if it's invalid, set it to nil to avoid errors and corrupted json
				txdata = nil
			}

			// get destination address from calldata
			dest, err := comm.ParseDestinationFromCallData(userop.CallData)
			if err != nil {
				continue
			}

			log := &engine.Log{
				TxHash:    signedTxHash,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
				Nonce:     userop.Nonce.Int64(),
				Sender:    userop.Sender.Hex(),
				To:        dest.Hex(),
				Value:     common.Big0,
				Data:      data,
				ExtraData: txdata,
				Status:    engine.LogStatusSending,
			}

			log.Hash = log.GenerateUniqueHash()

			err = ldb.AddLog(log)
			if err != nil {
				println("error adding log", err.Error())
			}

			// broadcast updates to connected clients
			s.pools.BroadcastMessage(engine.WSMessageTypeNew, log)

			insertedLogs[txm.Paymaster] = append(insertedLogs[txm.Paymaster], log)
		}

		// Send the signed transaction
		err = s.evm.SendTransaction(signedTx)
		if err != nil {
			// If there's an error, check if it's an RPC error
			e, ok := err.(rpc.Error)
			if ok && e.ErrorCode() != -32000 {
				// If it's an RPC error and the error code is not -32000, remove the sending transfer and return the error
				for _, logs := range insertedLogs {
					for _, log := range logs {
						ldb.RemoveLog(log.Hash)

						// broadcast updates to connected clients
						s.pools.BroadcastMessage(engine.WSMessageTypeRemove, log)
					}
				}

				invalid = append(invalid, msgs...)
				for range msgs {
					errors = append(errors, err)
				}

				// remove from inProgress
				s.mu.Lock()
				s.inProgress[entrypoint] = comm.Filter(s.inProgress[entrypoint], func(s string) bool {
					return s != signedTxHash
				})
				s.mu.Unlock()
				continue
			}

			if !strings.Contains(e.Error(), "insufficient funds") {
				// If the error is not about insufficient funds, remove the sending transfer and return the error
				for _, logs := range insertedLogs {
					for _, log := range logs {
						ldb.RemoveLog(log.Hash)

						// broadcast updates to connected clients
						s.pools.BroadcastMessage(engine.WSMessageTypeRemove, log)
					}
				}

				invalid = append(invalid, msgs...)
				for range msgs {
					errors = append(errors, err)
				}

				// remove from inProgress
				s.mu.Lock()
				s.inProgress[entrypoint] = comm.Filter(s.inProgress[entrypoint], func(s string) bool {
					return s != signedTxHash
				})
				s.mu.Unlock()
				continue
			}

			for _, logs := range insertedLogs {
				for _, log := range logs {
					ldb.SetStatus(log.Hash, string(engine.LogStatusFail))

					// broadcast updates to connected clients
					log.Status = engine.LogStatusFail
					s.pools.BroadcastMessage(engine.WSMessageTypeUpdate, log)
				}
			}

			// Return the error about insufficient funds
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}

			// remove from inProgress
			s.mu.Lock()
			s.inProgress[entrypoint] = comm.Filter(s.inProgress[entrypoint], func(s string) bool {
				return s != signedTxHash
			})
			s.mu.Unlock()
			continue
		}

		// Respond to the messages with the tx hash
		for _, msg := range msgs {
			msg.Respond(signedTxHash, nil)
		}

		for _, logs := range insertedLogs {
			for _, log := range logs {
				err := ldb.SetStatus(log.Hash, string(engine.LogStatusPending))
				if err != nil {
					ldb.RemoveLog(log.Hash)

					// broadcast updates to connected clients
					s.pools.BroadcastMessage(engine.WSMessageTypeRemove, log)
				}
			}
		}

		go func() {
			// async wait for the transaction to be mined
			err = s.evm.WaitForTx(signedTx, 16)
			if err != nil {
				for _, logs := range insertedLogs {
					for _, log := range logs {
						ldb.RemoveLog(log.Hash)

						// broadcast updates to connected clients
						s.pools.BroadcastMessage(engine.WSMessageTypeRemove, log)
					}
				}
			}

			// remove from inProgress
			s.mu.Lock()
			s.inProgress[entrypoint] = comm.Filter(s.inProgress[entrypoint], func(s string) bool {
				return s != signedTxHash
			})
			s.mu.Unlock()
		}()
	}

	return invalid, errors
}
