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
	return hexutil.EncodeBig(s.chainId), nil
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
			// If UserOp has a TxHash, get the real receipt from chain
			if userOp.TxHash != nil && *userOp.TxHash != "" {
				var result any
				txHashParams, _ := json.Marshal([]string{*userOp.TxHash})
				err := s.evm.Call("eth_getTransactionReceipt", &result, txHashParams)
				if err != nil {
					return nil, err
				}
				return result, nil
			}
			// No TxHash yet - return null (standard blockchain behavior for unknown/pending tx)
			return nil, nil
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
