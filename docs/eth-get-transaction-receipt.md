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
     ├── Found with TxHash ──► Forward to chain RPC using TxHash
     │
     ├── Found without TxHash ──► Return null (pending)
     │
     └── Not found ──► Forward to chain RPC (standard behavior)
```

### Lookup Priority

1. **UserOp Database Lookup**: The hash is first checked against the internal UserOp database
2. **TxHash Resolution**: If a UserOp is found and has an associated `TxHash`, the real receipt is fetched from the chain
3. **Chain RPC Fallback**: If not found in the database, the request is forwarded to the underlying chain RPC

## Response Behavior

| Scenario | Response |
|----------|----------|
| UserOp found with TxHash | Real chain receipt (same as querying by TxHash directly) |
| UserOp found, no TxHash yet | `null` (standard blockchain behavior for pending/unknown tx) |
| Hash not in UserOpDB | Forward to chain RPC |

### Key Behavior

When a UserOp has been submitted to the chain and has an associated `TxHash`:
- The **real chain receipt** is returned, not a synthetic one
- The `transactionHash` in the response will be the **bundled TxHash**, not the UserOp hash
- Querying by UserOp hash or by TxHash returns **identical responses**

This ensures consistency: clients can use either hash to get the same receipt data.

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

### Response (Mined)

When the UserOp has been bundled and mined, the real chain receipt is returned:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "transactionHash": "0xabcd...bundledTxHash",
    "from": "0xBundlerAddress",
    "to": "0xEntryPointAddress",
    "status": "0x1",
    "blockHash": "0xActualBlockHash",
    "blockNumber": "0x123",
    "transactionIndex": "0x0",
    "gasUsed": "0x5208",
    "cumulativeGasUsed": "0x5208",
    "effectiveGasPrice": "0x3b9aca00",
    "contractAddress": null,
    "logs": [...],
    "logsBloom": "0x...",
    "type": "0x2"
  }
}
```

### Response (Pending)

When the UserOp exists but hasn't been submitted to the chain yet:

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
3. **Real Receipts**: Successfully mined operations return the actual chain receipt with full data
4. **Consistent Responses**: Querying by UserOp hash or TxHash returns identical receipts
5. **Transparent Fallback**: Unknown hashes are transparently forwarded to the chain RPC

## Notes

- The `transactionHash` in the receipt is the bundled TxHash, not the UserOp hash
- The `logs` array contains all logs from the bundled transaction (which may include multiple UserOps)
- To filter logs specific to a UserOp, use the logs API with the UserOp sender address
