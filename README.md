# App Engine

App Engine is a Go application designed to simplify and enhance blockchain interactions on EVM-compatible chains. It aims to create great user experiences on EVM-compatible blockchains by providing a unified solution for reading and writing data, as well as handling event-based interactions.

## Purpose

Engine addresses several key challenges in blockchain development:

1. **Simplified Transactions**: Submitting abstracted transactions (user operations) becomes easier.
2. **Chronological Data Reading**: Efficiently read data in chronological order from the blockchain.
3. **Event Handling**: React to on-chain events through WebSockets (for application displays) or webhooks (for system responses).
4. **Data Enrichment**: Attach additional off-chain data to enhance use cases (e.g., transaction descriptions for ERC20 transfers).
5. **Real-time Interaction**: Make transactions and their data available before they are confirmed on-chain, enabling immediate user feedback and interaction.

## Features

- [x] Bundler
  - [x] RPC calls through REST Endpoints
    - [x] pm_sponsorUserOperation
    - [x] pm_ooSponsorUserOperation
    - [x] eth_sendUserOperation
    - [x] eth_chainId
  - [ ] RPC calls through WebSocket
    - [ ] pm_sponsorUserOperation
    - [ ] pm_ooSponsorUserOperation
    - [ ] eth_sendUserOperation
    - [ ] eth_chainId
- [ ] Smart Contract Logs
  - [x] Endpoints
    - [x] Fetch in a date range
  - [ ] WebSocket
    - [ ] Listen by Contract + Event Signature + Topic name/value (optional)
  - [x] Indexing
    - [x] Listen by Contract + Event Signature
  - [ ] Mechanism to automate requests to start indexing
    - [ ] Manually for system admins
    - [ ] By listening to a Smart Contract (people could pay to start indexing)
  - [x] Store in DB
    - [x] Hash
    - [x] Created at
    - [x] Destination Contract Address
    - [x] Amount
    - [x] Status
    - [x] Topics (as JSON)
    - [x] Extra Data (as JSON)
  - [ ] Webhooks (make a network request based on an event being triggered)

## About Citizen Wallet

Citizen Wallet is an open-source project focused on improving blockchain user experiences. Engine is a core component of this ecosystem.

- Website: [https://citizenwallet.xyz](https://citizenwallet.xyz)
- Discord: [https://discord.citizenwallet.xyz](https://discord.citizenwallet.xyz)
- X: [@citizenwallet](https://x.com/citizenwallet)

Join our community to contribute, ask questions, or learn more about the project!
