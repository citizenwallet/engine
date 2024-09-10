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

- [ ] Indexer/Bundler
  - [ ] Listen to Event X
    - [ ] Smart Contract
      - [ ] Listen by community_sponsor
      - [ ] Enable listening to Event X based on events from contract
        - [ ] Check price <= value that was sent with request
        - [ ] Check owner
      - [ ] Disable listening to Event X based on events from contract
        - [ ] Check owner
      - [ ] Set a price to enable listening
        - [ ] Check contract owner
      - [ ] Contract owner can withdraw
      - [ ] Set withdrawal address
        - [ ] Check contract owner
    - [ ] Store in DB
      - [ ] Destination Contract Address
      - [ ] Amount
      - [ ] Event Data (parsed as JSON if possible)
      - [ ] Data (extra data that can be attached as JSON)
    - [ ] Webhooks (make a network request based on an event being triggered)
  - [ ] Endpoints
    - [x] Fetch in a date range
    - [ ] Submit user operations against Paymaster Z
  - [ ] WebSocket (/community_sponsor)
    - [ ] Listen to Event X on Contract Y
    - [ ] Submit user operations against Paymaster Z
  - [ ] User Operation Queue
    - [ ] Store in DB
      - [ ] Hash
      - [ ] Created at
      - [ ] Status (processing, submitted, confirmed, reverted)
      - [ ] User operation (JSON)
    - [ ] Endpoints
      - [ ] pm_sponsorUserOperation
      - [ ] pm_ooSponsorUserOperation
      - [ ] eth_sendUserOperation
      - [ ] eth_chainId

## About Citizen Wallet

Citizen Wallet is an open-source project focused on improving blockchain user experiences. Engine is a core component of this ecosystem.

- Website: [https://citizenwallet.xyz](https://citizenwallet.xyz)
- Discord: [https://discord.citizenwallet.xyz](https://discord.citizenwallet.xyz)
- X: [@citizenwallet](https://x.com/citizenwallet)

Join our community to contribute, ask questions, or learn more about the project!
