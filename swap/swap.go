// Package swap exposes the Hinkal SDK swap- and bridge-quote helpers.
package swap

import (
	"github.com/gioeba/go_sdk_test/internal/functions/integrations"
	pretx "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
)

type (
	UniswapPrice = web3.UniswapPrice
	OdosPrice    = web3.OdosPrice
	OneInchPrice = web3.OneInchPrice
	OKXPrice     = pretx.OKXPrice

	EVMSwapPrices    = integrations.EVMSwapPrices
	SolanaSwapPrices = integrations.SolanaSwapPrices
)

var (
	GetUniswapPrice = web3.GetUniswapPrice
	GetOdosPrice    = web3.GetOdosPrice
	GetOneInchPrice = web3.GetOneInchPrice
	GetLifiPrice    = web3.GetLifiPrice
	GetOKXPrice     = pretx.GetOKXPrice

	GetEVMSwapPrices    = integrations.GetEVMSwapPrices
	GetSolanaSwapPrices = integrations.GetSolanaSwapPrices

	GetExternalSwapAddress = pretx.GetExternalSwapAddress
)
