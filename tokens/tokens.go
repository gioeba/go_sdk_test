// Package tokens exposes the Hinkal SDK token-amount and ERC20 resolution helpers.
package tokens

import "github.com/gioeba/go_sdk_test/internal/functions/web3"

var (
	GetAmountInWei         = web3.GetAmountInWei
	GetAmountInToken       = web3.GetAmountInToken
	GetAmountWithPrecision = web3.GetAmountWithPrecision

	ResolveERC20Tokens       = web3.ResolveERC20Tokens
	ResolveERC20TokenStrict  = web3.ResolveERC20TokenStrict
	ResolveERC20TokensStrict = web3.ResolveERC20TokensStrict
)
