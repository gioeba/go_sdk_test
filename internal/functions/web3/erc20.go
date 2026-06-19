package web3

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

// ResolveERC20Tokens maps token addresses to their registry entries on the given chain,
// falling back to a minimal token for addresses missing from the registry.
func ResolveERC20Tokens(chainID int, erc20Addresses []string) []types.ERC20Token {
	tokens := make([]types.ERC20Token, len(erc20Addresses))
	for i, address := range erc20Addresses {
		if token := constants.GetERC20Token(address, chainID); token != nil {
			tokens[i] = *token
			tokens[i].ChainID = chainID
			continue
		}
		tokens[i] = types.ERC20Token{ChainID: chainID, Erc20TokenAddress: address}
	}
	return tokens
}

func ResolveERC20TokenStrict(ctx context.Context, chainID int, erc20Address string) (types.ERC20Token, error) {
	tokens, err := ResolveERC20TokensStrict(ctx, chainID, []string{erc20Address})
	if err != nil {
		return types.ERC20Token{}, err
	}
	return tokens[0], nil
}

func ResolveERC20TokensStrict(ctx context.Context, chainID int, erc20Addresses []string) ([]types.ERC20Token, error) {
	if len(erc20Addresses) == 0 {
		return nil, nil
	}

	apiTokens, err := api.GetTokensInfo(ctx, chainID, erc20Addresses, false)
	if err != nil {
		return nil, err
	}
	if len(apiTokens) != len(erc20Addresses) {
		return nil, fmt.Errorf("web3: expected %d token records, got %d", len(erc20Addresses), len(apiTokens))
	}

	tokens := make([]types.ERC20Token, len(erc20Addresses))
	for i, token := range apiTokens {
		if token == nil {
			return nil, fmt.Errorf("web3: token not found: %s on chain %d", erc20Addresses[i], chainID)
		}
		tokens[i] = *token
		tokens[i].ChainID = chainID
	}
	return tokens, nil
}

const erc20ABIJSON = `[
  {"name":"allowance","type":"function","stateMutability":"view","inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
  {"name":"approve","type":"function","stateMutability":"nonpayable","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]}
]`

var erc20ABI = func() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(erc20ABIJSON))
	if err != nil {
		panic(fmt.Sprintf("web3: invalid erc20 abi: %v", err))
	}
	return parsed
}()

// Erc20Allowance reads token.allowance(owner, spender) via eth_call.
func Erc20Allowance(ctx context.Context, client *ethclient.Client, token, owner, spender common.Address) (*big.Int, error) {
	data, err := erc20ABI.Pack("allowance", owner, spender)
	if err != nil {
		return nil, err
	}
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	values, err := erc20ABI.Unpack("allowance", out)
	if err != nil {
		return nil, err
	}
	allowance, ok := values[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("web3: unexpected allowance type %T", values[0])
	}
	return allowance, nil
}

// Erc20ApproveCalldata encodes approve(spender, amount).
func Erc20ApproveCalldata(spender common.Address, amount *big.Int) ([]byte, error) {
	return erc20ABI.Pack("approve", spender, amount)
}
