package providers

import (
	"github.com/gioeba/go_sdk_test/types"
)

var (
	_ types.IProviderAdapter = (*EthersProviderAdapter)(nil)
	_ types.IProviderAdapter = (*SolanaProviderAdapter)(nil)
	_ types.IProviderAdapter = (*TronProviderAdapter)(nil)
)
