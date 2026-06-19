// Package utxo exposes the Hinkal SDK UTXO type and decoding helpers.
package utxo

import (
	onchain "github.com/gioeba/go_sdk_test/internal/functions/onchainutxos"
	impl "github.com/gioeba/go_sdk_test/internal/utxo"
)

type Utxo = impl.Utxo

var (
	NewUtxo    = impl.NewUtxo
	CreateFrom = impl.CreateFrom
)

var (
	DecodeFromReceipt           = onchain.DecodeFromReceipt
	DecodeSolanaFromTransaction = onchain.DecodeSolanaFromTransaction
)
