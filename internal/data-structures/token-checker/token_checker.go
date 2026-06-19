package tokenchecker

import (
	"strings"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

func IsBeefyStakeToken(token types.ERC20Token) bool {
	return strings.Contains(token.Name, "-Boost") && strings.Contains(token.Symbol, "-Boost")
}

func IsStakeToken(token types.ERC20Token) bool {
	return IsBeefyStakeToken(token)
}

func IsAaveToken(token types.ERC20Token) bool {
	return token.AaveToken
}

func IsKinzaToken(token types.ERC20Token) bool {
	return strings.HasPrefix(token.Name, "Kinza")
}

func IsPotentiallyVolatile(token types.ERC20Token) bool {
	if token.Erc20TokenAddress == constants.ZeroAddress {
		return false
	}
	return token.IsVolatile ||
		(!constants.IsSolanaLike(token.ChainID) &&
			!constants.IsTronLike(token.ChainID) &&
			(token.BalanceStorageOffset == nil || token.AllowanceStorageOffset == nil))
}
