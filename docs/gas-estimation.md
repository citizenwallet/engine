# Gas Estimation Strategy

This document describes how the engine calculates gas fees for EIP-1559 transactions to ensure fast block inclusion.

## Overview

The `GetFeeEstimates()` function in `internal/ethrequest/ethrequest.go` uses `eth_feeHistory` to analyze recent blocks and calculate optimal gas fees. The strategy is optimized for **fast inclusion** - paying a premium to be prioritized by validators.

## How It Works

### 1. Fetch Fee History

```go
feeHistory, err := e.FeeHistory(5, []float64{90})
```

- Fetches the last **5 blocks** of fee data
- Requests the **90th percentile** of priority fees (tips) from each block
- This tells us what the top 10% of transactions paid

### 2. Calculate Priority Fee (Tip)

The priority fee is what validators receive as incentive to include your transaction.

| Step | Logic |
|------|-------|
| Base | Take the **maximum** 90th percentile priority fee from recent blocks |
| Buffer | Add **+50%** on top to outbid competitors |
| Minimum | Ensure at least **5x the base fee** for guaranteed prioritization |

```go
// Add 50% buffer on top of 90th percentile
priorityBuffer := new(big.Int).Div(maxRecentPriority, big.NewInt(2))
maxPriorityFeePerGas = new(big.Int).Add(maxRecentPriority, priorityBuffer)

// Ensure minimum of 5x base fee
minPriority := new(big.Int).Mul(latestBaseFee, big.NewInt(5))
if maxPriorityFeePerGas.Cmp(minPriority) < 0 {
    maxPriorityFeePerGas = minPriority
}
```

### 3. Calculate Max Fee

The max fee is the absolute maximum you're willing to pay per gas unit.

```go
// baseFee * 2 + priorityFee
bufferedBaseFee := new(big.Int).Mul(latestBaseFee, big.NewInt(2))
maxFeePerGas = new(big.Int).Add(bufferedBaseFee, maxPriorityFeePerGas)
```

- **2x base fee buffer** handles potential congestion spikes over the next few blocks
- You only pay `baseFee + priorityFee`, the rest is headroom

## Example Calculation

Given a base fee of `0.000086 Gwei`:

| Fee Component | Calculation | Result |
|---------------|-------------|--------|
| Min Priority Fee | 0.000086 × 5 | ~0.00043 Gwei |
| Base Fee Buffer | 0.000086 × 2 | ~0.000172 Gwei |
| Max Fee | 0.000172 + 0.00043 | ~0.0006 Gwei |

## Gas Limit

The gas limit (how much gas the transaction can use) is calculated separately:

1. **Estimate**: Call `eth_estimateGas` with the transaction data
2. **Fallback**: If estimation fails, use average of recent successful estimates
3. **Buffer**: Add **20%** buffer for safety

```go
gasBuffer := gasLimit / 5  // 20% buffer
tx.Gas = gasLimit + gasBuffer
```

## Retry Mechanism

The `extraGas` parameter in `NewTx()` allows increasing fees for retries:

```go
if extraGas > 0 {
    multiplier := big.NewInt(int64(1 + extraGas))
    gasFeeCap = new(big.Int).Mul(maxFeePerGas, multiplier)
    gasTipCap = new(big.Int).Mul(maxPriorityFeePerGas, multiplier)
}
```

- `extraGas=1` doubles the fees
- `extraGas=2` triples the fees
- etc.

## Network Compatibility

This strategy works on all EIP-1559 compatible networks:

- Gnosis Chain (xDai)
- Base
- Arbitrum
- Optimism
- Ethereum Mainnet

The `eth_feeHistory` API automatically adapts to each network's fee levels.

## Trade-offs

| Aspect | Current Strategy |
|--------|------------------|
| Speed | Optimized for next-block inclusion |
| Cost | Pays premium (5x+ base fee as priority) |
| Reliability | High - outbids 90%+ of transactions |

To reduce costs at the expense of speed, adjust:
- Use lower percentile (50th instead of 90th)
- Reduce priority multiplier (2x instead of 5x)
- Remove the 50% priority buffer
