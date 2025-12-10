# Modified `eth_getTransactionReceipt`

This document explains how the engine's modified `eth_getTransactionReceipt` RPC method works, enabling clients to track ERC-4337 User Operations using the same familiar interface as regular Ethereum transactions.

## Overview

The standard `eth_getTransactionReceipt` only works with on-chain transaction hashes. However, when submitting User Operations (UserOps) through the engine, clients receive a **UserOp hash** rather than a transaction hash. The UserOp hash is computed locally before the operation is batched and submitted to the chain.

Our modified implementation bridges this gap by allowing clients to query receipts using **either**:
- A UserOp hash (tracked internally by the engine)
- A regular transaction hash (forwarded to the chain RPC)

## How It Works

### Request Flow

```
Client Request
     │
     ▼
┌─────────────────────────────────┐
│  Parse hash from params         │
└─────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────┐
│  Check UserOpDB for hash        │
└─────────────────────────────────┘
     │
     ├── Found ──► Build synthetic receipt from stored UserOp
     │
     └── Not found ──► Forward to chain RPC (standard behavior)
```

### Lookup Priority

1. **UserOp Database Lookup**: The hash is first checked against the internal UserOp database
2. **Chain RPC Fallback**: If not found in the database, the request is forwarded to the underlying chain RPC

## Synthetic Receipt Structure

When a UserOp is found in the database, a synthetic receipt is constructed that mimics a standard Ethereum transaction receipt.

### Status Handling

| UserOp Status | Receipt Behavior |
|---------------|------------------|
| `pending`     | Returns `null` (matches standard pending tx behavior) |
| `submitted`   | Returns `null` (transaction not yet mined) |
| `success`     | Returns receipt with `status: "0x1"` |
| `reverted`    | Returns receipt with `status: "0x0"` |
| `timeout`     | Returns receipt with `status: "0x0"` |

### Receipt Fields

The synthetic receipt includes the following fields:

| Field | Value |
|-------|-------|
| `transactionHash` | The UserOp hash |
| `from` | The sender address from the UserOp |
| `to` | The EntryPoint contract address |
| `status` | `"0x1"` for success, `"0x0"` for failure |
| `contractAddress` | `null` |
| `logs` | Empty array `[]` |
| `logsBloom` | Zero-filled bloom filter |
| `type` | `"0x2"` (EIP-1559 transaction type) |

### Block Information

If the UserOp has been successfully submitted and has an associated transaction hash (`TxHash`), the engine fetches actual block information from the chain:

- `blockHash`
- `blockNumber`
- `transactionIndex`
- `gasUsed`
- `cumulativeGasUsed`
- `effectiveGasPrice`

If no transaction hash is available (or the chain lookup fails), placeholder values are used:
- `blockHash`: Zero hash
- `blockNumber`: `"0x0"`
- All gas fields: `"0x0"`

## Example Usage

### Request

```json
{
  "jsonrpc": "2.0",
  "method": "eth_getTransactionReceipt",
  "params": ["0x1234...userOpHash"],
  "id": 1
}
```

### Response (Success)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "transactionHash": "0x1234...userOpHash",
    "from": "0xSenderAddress",
    "to": "0xEntryPointAddress",
    "status": "0x1",
    "blockHash": "0xActualBlockHash",
    "blockNumber": "0x123",
    "transactionIndex": "0x0",
    "gasUsed": "0x5208",
    "cumulativeGasUsed": "0x5208",
    "effectiveGasPrice": "0x3b9aca00",
    "contractAddress": null,
    "logs": [],
    "logsBloom": "0x000...000",
    "type": "0x2"
  }
}
```

### Response (Pending)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": null
}
```

## Benefits

1. **Unified Interface**: Clients can use the same RPC method for both UserOps and regular transactions
2. **Familiar Semantics**: Pending operations return `null`, matching standard Ethereum behavior
3. **Real Block Data**: Successfully mined operations include actual on-chain block information
4. **Transparent Fallback**: Unknown hashes are transparently forwarded to the chain RPC

## Limitations

- The `logs` array is always empty in synthetic receipts (UserOp logs are available through the logs API)
- Gas values may be placeholders if the underlying transaction hasn't been mined yet
- The `transactionHash` in the receipt is the UserOp hash, not the bundled transaction hash
