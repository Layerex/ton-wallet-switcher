# ton-wallet-switcher

An utility for managing multiple [TON](https://ton.org/) Wallet wallets

## Installation

```sh
go build
sudo install ton-wallet-switcher /usr/local/bin
```

## Usage

``` text
Usage: ton-wallet-switcher [COMMAND] [WALLET]

An utility for managing multiple TON Wallet wallets

Commands:
  init            Initialize: find all wallets and ask user to describe them
  status          List wallets
  switch [WALLET] Switch to another wallet
  edit [WALLET]   Edit wallet name and description
  add [WALLET]    Add an existing wallet directory or create a new one
  forget [WALLET] Forget about wallet
  config          Get this utility config path
  directory       Get TON Wallet directory path
  help            Print this help
```
