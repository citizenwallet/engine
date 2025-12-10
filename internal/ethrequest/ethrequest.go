package ethrequest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	ETHEstimateGas        = "eth_estimateGas"
	ETHSendRawTransaction = "eth_sendRawTransaction"
	ETHSign               = "eth_sign"
	ETHChainID            = "eth_chainId"
)

type EthBlock struct {
	Number    string `json:"number"`
	Timestamp string `json:"timestamp"`
}

// FeeHistoryResult holds the response from eth_feeHistory
type FeeHistoryResult struct {
	OldestBlock   string     `json:"oldestBlock"`
	BaseFeePerGas []string   `json:"baseFeePerGas"`
	GasUsedRatio  []float64  `json:"gasUsedRatio"`
	Reward        [][]string `json:"reward"` // priority fees at requested percentiles
}

type EthService struct {
	rpc    *rpc.Client
	client *ethclient.Client
	ctx    context.Context

	// Gas estimate tracking for future-nonce transactions
	gasEstimates []uint64
	gasEstMu     sync.Mutex
}

func (e *EthService) Context() context.Context {
	return e.ctx
}

func NewEthService(ctx context.Context, endpoint string) (*EthService, error) {
	rpc, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, err
	}

	client := ethclient.NewClient(rpc)

	return &EthService{
		rpc:          rpc,
		client:       client,
		ctx:          ctx,
		gasEstimates: make([]uint64, 0, 5),
	}, nil
}

func (e *EthService) Close() {
	e.client.Close()
}

// trackGasEstimate stores a successful gas estimate for future reference
func (e *EthService) trackGasEstimate(gasLimit uint64) {
	e.gasEstMu.Lock()
	defer e.gasEstMu.Unlock()

	// Add the new estimate
	e.gasEstimates = append(e.gasEstimates, gasLimit)

	// Keep only the last 5 estimates
	if len(e.gasEstimates) > 5 {
		e.gasEstimates = e.gasEstimates[len(e.gasEstimates)-5:]
	}
}

// getAverageGasEstimate returns the average of recent gas estimates
// Returns 0 if no estimates are available
func (e *EthService) getAverageGasEstimate() uint64 {
	e.gasEstMu.Lock()
	defer e.gasEstMu.Unlock()

	if len(e.gasEstimates) == 0 {
		return 0
	}

	var sum uint64
	for _, estimate := range e.gasEstimates {
		sum += estimate
	}

	return sum / uint64(len(e.gasEstimates))
}

func (e *EthService) BlockTime(number *big.Int) (uint64, error) {
	// Some blockchains have a slightly different format than Ethereum Blocks, so we need to use a custom Block struct
	var blk *EthBlock
	err := e.rpc.Call(&blk, "eth_getBlockByNumber", fmt.Sprintf("0x%s", number.Text(16)), true)
	if err != nil {
		return 0, err
	}

	if blk == nil {
		return 0, errors.New("block not found")
	}

	v, err := hexutil.DecodeUint64(blk.Timestamp)
	if err != nil {
		return 0, err
	}

	return v, nil
}

func (e *EthService) Backend() bind.ContractBackend {
	return e.client
}

func (e *EthService) CallContract(call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	return e.client.CallContract(e.ctx, call, blockNumber)
}

func (e *EthService) ListenForLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) error {
	for {
		sub, err := e.client.SubscribeFilterLogs(ctx, q, ch)
		if err != nil {
			log.Default().Println("error subscribing to logs", err.Error())

			<-time.After(1 * time.Second)

			continue
		}

		select {
		case <-ctx.Done():
			log.Default().Println("context done, unsubscribing")
			sub.Unsubscribe()

			return ctx.Err()
		case err := <-sub.Err():
			// subscription error, try and re-subscribe
			log.Default().Println("subscription error", err.Error())
			sub.Unsubscribe()

			<-time.After(1 * time.Second)

			continue
		}
	}
}

func (e *EthService) CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error) {
	return e.client.CodeAt(e.ctx, account, blockNumber)
}

func (e *EthService) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	return e.client.NonceAt(e.ctx, account, blockNumber)
}

func (e *EthService) BaseFee() (*big.Int, error) {
	// Get the latest block header
	header, err := e.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	return header.BaseFee, nil
}

// FeeHistory fetches historical fee data using eth_feeHistory
// blockCount: number of blocks to fetch (typically 4-5)
// percentiles: priority fee percentiles to fetch (e.g., [25, 50, 75])
func (e *EthService) FeeHistory(blockCount int, percentiles []float64) (*FeeHistoryResult, error) {
	var result FeeHistoryResult
	err := e.rpc.Call(&result, "eth_feeHistory", hexutil.EncodeUint64(uint64(blockCount)), "latest", percentiles)
	if err != nil {
		return nil, fmt.Errorf("error calling eth_feeHistory: %w", err)
	}
	return &result, nil
}

// GetFeeEstimates returns recommended maxFeePerGas and maxPriorityFeePerGas
// based on recent block history using eth_feeHistory
func (e *EthService) GetFeeEstimates() (maxFeePerGas, maxPriorityFeePerGas *big.Int, err error) {
	// Fetch 5 recent blocks with 50th percentile (median) priority fees
	feeHistory, err := e.FeeHistory(5, []float64{50})
	if err != nil {
		return nil, nil, fmt.Errorf("error getting fee history: %w", err)
	}

	if len(feeHistory.BaseFeePerGas) == 0 {
		return nil, nil, errors.New("no base fee data in fee history")
	}

	// Get the latest base fee (last element is for the pending block)
	latestBaseFeeHex := feeHistory.BaseFeePerGas[len(feeHistory.BaseFeePerGas)-1]
	latestBaseFee, err := hexutil.DecodeBig(latestBaseFeeHex)
	if err != nil {
		return nil, nil, fmt.Errorf("error decoding base fee: %w", err)
	}

	// Calculate median priority fee from recent blocks
	var priorityFees []*big.Int
	for _, rewards := range feeHistory.Reward {
		if len(rewards) > 0 {
			fee, err := hexutil.DecodeBig(rewards[0]) // 50th percentile
			if err == nil && fee.Sign() > 0 {
				priorityFees = append(priorityFees, fee)
			}
		}
	}

	// Calculate average of priority fees from recent blocks
	var avgPriorityFee *big.Int
	if len(priorityFees) > 0 {
		sum := big.NewInt(0)
		for _, fee := range priorityFees {
			sum.Add(sum, fee)
		}
		avgPriorityFee = new(big.Int).Div(sum, big.NewInt(int64(len(priorityFees))))
	} else {
		// Fallback to eth_maxPriorityFeePerGas if no reward data
		avgPriorityFee, err = e.MaxPriorityFeePerGas()
		if err != nil {
			return nil, nil, fmt.Errorf("error getting max priority fee: %w", err)
		}
	}

	// Add 20% buffer to priority fee for faster inclusion
	priorityBuffer := new(big.Int).Div(avgPriorityFee, big.NewInt(5))
	maxPriorityFeePerGas = new(big.Int).Add(avgPriorityFee, priorityBuffer)

	// Calculate maxFeePerGas: baseFee * 1.25 + priorityFee
	// The 25% buffer accounts for 1-2 blocks of base fee fluctuation
	baseFeeBuffer := new(big.Int).Div(latestBaseFee, big.NewInt(4))
	bufferedBaseFee := new(big.Int).Add(latestBaseFee, baseFeeBuffer)
	maxFeePerGas = new(big.Int).Add(bufferedBaseFee, maxPriorityFeePerGas)

	return maxFeePerGas, maxPriorityFeePerGas, nil
}

func (e *EthService) EstimateGasPrice() (*big.Int, error) {
	return e.client.SuggestGasPrice(e.ctx)
}

func (e *EthService) EstimateGasLimit(msg ethereum.CallMsg) (uint64, error) {
	gasLimit, err := e.client.EstimateGas(e.ctx, msg)
	if err != nil {
		// Log more details about the error
		fmt.Printf("EstimateGasLimit error type: %T\n", err)
		fmt.Printf("EstimateGasLimit error details: %+v\n", err)

		// Try to extract more information if it's an RPC error
		if rpcErr, ok := err.(rpc.Error); ok {
			fmt.Printf("RPC error code: %d\n", rpcErr.ErrorCode())
			fmt.Printf("RPC error message: %s\n", rpcErr.Error())
		}
	}
	return gasLimit, err
}

func (e *EthService) NewTx(nonce uint64, from, to common.Address, data []byte, extraGas int) (*types.Transaction, error) {
	// Get fee estimates based on recent block history (similar to MetaMask)
	maxFeePerGas, maxPriorityFeePerGas, err := e.GetFeeEstimates()
	if err != nil {
		return nil, fmt.Errorf("error getting fee estimates: %w", err)
	}

	// Prepare the call message for gas estimation
	msg := ethereum.CallMsg{
		From:     from,
		To:       &to,
		Gas:      0,
		GasPrice: nil,
		Value:    nil,
		Data:     data,
	}

	// Estimate gas limit
	gasLimit, err := e.EstimateGasLimit(msg)
	if err != nil {
		// Gas estimation failed - fall back to average of recent successful estimates
		avgGas := e.getAverageGasEstimate()
		if avgGas > 0 {
			gasLimit = avgGas
			fmt.Printf("Gas estimation failed, using average gas limit %d from recent estimates\n", gasLimit)
		} else {
			gasLimit = 500000
			fmt.Printf("Gas estimation failed with no historical data, using fallback gas limit %d\n", gasLimit)
		}
	} else {
		e.trackGasEstimate(gasLimit)
	}

	// Add 20% gas buffer for safety
	gasBuffer := gasLimit / 5

	// Apply extraGas multiplier if specified (for retries with higher fees)
	gasFeeCap := maxFeePerGas
	gasTipCap := maxPriorityFeePerGas
	if extraGas > 0 {
		multiplier := big.NewInt(int64(1 + extraGas))
		gasFeeCap = new(big.Int).Mul(maxFeePerGas, multiplier)
		gasTipCap = new(big.Int).Mul(maxPriorityFeePerGas, multiplier)
	}

	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       gasLimit + gasBuffer,
		To:        &to,
		Value:     common.Big0,
		Data:      data,
	})
	return tx, nil
}

func (e *EthService) EstimateFullGas(from common.Address, tx *types.Transaction) (uint64, error) {

	msg := ethereum.CallMsg{
		From:       from,
		To:         tx.To(),
		Gas:        tx.Gas(),
		GasPrice:   tx.GasPrice(),
		GasFeeCap:  tx.GasFeeCap(),
		GasTipCap:  tx.GasTipCap(),
		Value:      tx.Value(),
		Data:       tx.Data(),
		AccessList: tx.AccessList(),
	}

	return e.client.EstimateGas(e.ctx, msg)
}

func (e *EthService) SendTransaction(tx *types.Transaction) error {
	return e.client.SendTransaction(e.ctx, tx)
}

func (e *EthService) MaxPriorityFeePerGas() (*big.Int, error) {
	var hexFee string
	err := e.rpc.Call(&hexFee, "eth_maxPriorityFeePerGas")
	if err != nil {
		return common.Big0, err
	}

	fee := new(big.Int)
	_, ok := fee.SetString(hexFee[2:], 16) // remove the "0x" prefix and parse as base 16
	if !ok {
		return nil, errors.New("invalid hex string")
	}

	return fee, nil
}

func (e *EthService) StorageAt(addr common.Address, slot common.Hash) ([]byte, error) {
	return e.client.StorageAt(e.ctx, addr, slot, nil)
}

func (e *EthService) ChainID() (*big.Int, error) {
	chid, err := e.client.ChainID(e.ctx)
	if err != nil {
		return nil, err
	}

	return chid, nil
}

func (e *EthService) Call(method string, result any, params json.RawMessage) error {
	var args []any

	if err := json.Unmarshal(params, &args); err != nil {
		return fmt.Errorf("failed to unmarshal request body: %w", err)
	}

	return e.client.Client().Call(result, method, args...)
}

func (e *EthService) LatestBlock() (*big.Int, error) {
	var blk *EthBlock
	err := e.rpc.Call(&blk, "eth_getBlockByNumber", "latest", true)
	if err != nil {
		return common.Big0, err
	}

	v, err := hexutil.DecodeBig(blk.Number)
	if err != nil {
		return common.Big0, err
	}
	return v, nil
}

func (e *EthService) FilterLogs(q ethereum.FilterQuery) ([]types.Log, error) {
	return e.client.FilterLogs(e.ctx, q)
}

func (e *EthService) WaitForTx(tx *types.Transaction, timeout int) error {
	// Create a context that will be canceled after 4 seconds
	ctx, cancel := context.WithTimeout(e.ctx, time.Duration(timeout)*time.Second)
	defer cancel() // Cancel the context when the function returns

	rcpt, err := bind.WaitMined(ctx, e.client, tx)
	if err != nil {
		return err
	}

	if rcpt.Status != types.ReceiptStatusSuccessful {
		return errors.New("tx failed")
	}

	return nil
}
