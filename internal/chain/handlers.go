package chain

import (
	"encoding/json"
	"math/big"
	"net/http"

	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/pkg/engine"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type Service struct {
	evm     engine.EVMRequester
	db      *db.DB
	chainId *big.Int
}

// NewService
func NewService(evm engine.EVMRequester, database *db.DB, chid *big.Int) *Service {
	return &Service{
		evm:     evm,
		db:      database,
		chainId: chid,
	}
}

func (s *Service) ChainId(r *http.Request) (any, error) {
	// Return the message ID
	return s.chainId.String(), nil
}

func (s *Service) EthCall(r *http.Request) (any, error) {

	var params json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		return nil, err
	}

	var result any
	err := s.evm.Call("eth_call", &result, params)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return result, nil
}

func (s *Service) EthBlockNumber(r *http.Request) (any, error) {

	var params json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		return nil, err
	}

	var result any
	err := s.evm.Call("eth_blockNumber", &result, params)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return result, nil
}

func (s *Service) EthGetBlockByNumber(r *http.Request) (any, error) {

	var params json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		return nil, err
	}

	var result any
	err := s.evm.Call("eth_getBlockByNumber", &result, params)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return result, nil
}

func (s *Service) EthMaxPriorityFeePerGas(r *http.Request) (any, error) {

	var params json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		return nil, err
	}

	var result any
	err := s.evm.Call("eth_maxPriorityFeePerGas", &result, params)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return result, nil

}

func (s *Service) EthGetTransactionReceipt(r *http.Request) (any, error) {
	var params json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		return nil, err
	}

	// Parse the hash from params (it's an array with one element)
	var hashParams []string
	if err := json.Unmarshal(params, &hashParams); err == nil && len(hashParams) > 0 {
		hash := hashParams[0]

		// Check if this is a user op hash in our database
		userOp, err := s.db.UserOpDB.GetUserOp(hash)
		if err == nil && userOp != nil {
			// Found a user op - return synthetic receipt based on status
			return s.buildSyntheticReceipt(userOp), nil
		}
	}

	// Not found in UserOpDB, fall through to chain RPC
	var result any
	err := s.evm.Call("eth_getTransactionReceipt", &result, params)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return result, nil
}

// buildSyntheticReceipt creates a transaction receipt-like response from a stored user operation
func (s *Service) buildSyntheticReceipt(userOp *db.StoredUserOp) map[string]interface{} {
	// For pending/submitted status, return null (standard behavior for pending txs)
	if userOp.Status == db.UserOpStatusPending || userOp.Status == db.UserOpStatusSubmitted {
		return nil
	}

	// Build a synthetic receipt
	receipt := map[string]interface{}{
		"transactionHash": userOp.UserOpHash,
		"from":            userOp.Sender,
		"to":              userOp.EntryPoint,
		"contractAddress": nil,
		"logs":            []interface{}{},
		"logsBloom":       "0x" + "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		"type":            "0x2",
	}

	// Set status based on user op status
	switch userOp.Status {
	case db.UserOpStatusSuccess:
		receipt["status"] = "0x1"
	case db.UserOpStatusReverted:
		receipt["status"] = "0x0"
	default:
		receipt["status"] = "0x0"
	}

	// If we have a real tx hash, try to get actual block info from the chain
	if userOp.TxHash != nil && *userOp.TxHash != "" {
		// Try to get the actual receipt from chain for block info
		var chainReceipt map[string]interface{}
		hashParams := []string{*userOp.TxHash}
		paramsJSON, _ := json.Marshal(hashParams)
		err := s.evm.Call("eth_getTransactionReceipt", &chainReceipt, paramsJSON)
		if err == nil && chainReceipt != nil {
			// Copy block info from chain receipt
			if blockHash, ok := chainReceipt["blockHash"]; ok {
				receipt["blockHash"] = blockHash
			}
			if blockNumber, ok := chainReceipt["blockNumber"]; ok {
				receipt["blockNumber"] = blockNumber
			}
			if transactionIndex, ok := chainReceipt["transactionIndex"]; ok {
				receipt["transactionIndex"] = transactionIndex
			}
			if gasUsed, ok := chainReceipt["gasUsed"]; ok {
				receipt["gasUsed"] = gasUsed
			}
			if cumulativeGasUsed, ok := chainReceipt["cumulativeGasUsed"]; ok {
				receipt["cumulativeGasUsed"] = cumulativeGasUsed
			}
			if effectiveGasPrice, ok := chainReceipt["effectiveGasPrice"]; ok {
				receipt["effectiveGasPrice"] = effectiveGasPrice
			}
		} else {
			// Fallback values if we can't get chain receipt
			receipt["blockHash"] = "0x0000000000000000000000000000000000000000000000000000000000000000"
			receipt["blockNumber"] = hexutil.EncodeBig(big.NewInt(0))
			receipt["transactionIndex"] = "0x0"
			receipt["gasUsed"] = "0x0"
			receipt["cumulativeGasUsed"] = "0x0"
			receipt["effectiveGasPrice"] = "0x0"
		}
	} else {
		// No tx hash yet, use placeholder values
		receipt["blockHash"] = "0x0000000000000000000000000000000000000000000000000000000000000000"
		receipt["blockNumber"] = hexutil.EncodeBig(big.NewInt(0))
		receipt["transactionIndex"] = "0x0"
		receipt["gasUsed"] = "0x0"
		receipt["cumulativeGasUsed"] = "0x0"
		receipt["effectiveGasPrice"] = "0x0"
	}

	return receipt
}

func (s *Service) EthGetTransactionCount(r *http.Request) (any, error) {

	var params json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		return nil, err
	}

	var result any
	err := s.evm.Call("eth_getTransactionCount", &result, params)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return result, nil
}

func (s *Service) EthEstimateGas(r *http.Request) (any, error) {

	var params json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		return nil, err
	}

	var result any
	err := s.evm.Call("eth_estimateGas", &result, params)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return result, nil
}

func (s *Service) EthGasPrice(r *http.Request) (any, error) {

	var params json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		return nil, err
	}

	var result any
	err := s.evm.Call("eth_gasPrice", &result, params)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return result, nil
}

func (s *Service) EthSendRawTransaction(r *http.Request) (any, error) {

	var params json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		return nil, err
	}

	var result any
	err := s.evm.Call("eth_sendRawTransaction", &result, params)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return result, nil
}
