package web3

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

var (
	errUniswapNoLiquidity = errors.New("web3: not enough Uniswap liquidity for token pair")
	errNoUniswapPrice     = errors.New("web3: failed to fetch Uniswap price")

	uniswapPoolFees = []int64{100, 500, 3000, 10000}

	uniswapQuoteABI = func() abi.ABI {
		const j = `[
		  {"name":"getPool","type":"function","stateMutability":"view","inputs":[{"name":"tokenA","type":"address"},{"name":"tokenB","type":"address"},{"name":"fee","type":"uint24"}],"outputs":[{"name":"","type":"address"}]},
		  {"name":"balanceOf","type":"function","stateMutability":"view","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
		  {"name":"quoteExactInputSingle","type":"function","stateMutability":"nonpayable","inputs":[{"name":"params","type":"tuple","components":[{"name":"tokenIn","type":"address"},{"name":"tokenOut","type":"address"},{"name":"amountIn","type":"uint256"},{"name":"fee","type":"uint24"},{"name":"sqrtPriceLimitX96","type":"uint160"}]}],"outputs":[{"name":"amountOut","type":"uint256"},{"name":"sqrtPriceX96After","type":"uint160"},{"name":"initializedTicksCrossed","type":"uint32"},{"name":"gasEstimate","type":"uint256"}]}
		]`
		parsed, err := abi.JSON(strings.NewReader(j))
		if err != nil {
			panic(fmt.Sprintf("web3: invalid uniswap quote abi: %v", err))
		}
		return parsed
	}()

	uint24Argument = func() abi.Arguments {
		t, err := abi.NewType("uint24", "", nil)
		if err != nil {
			panic(fmt.Sprintf("web3: invalid uint24 type: %v", err))
		}
		return abi.Arguments{{Type: t}}
	}()
)

type quoteExactInputSingleParams struct {
	TokenIn           common.Address
	TokenOut          common.Address
	AmountIn          *big.Int
	Fee               *big.Int
	SqrtPriceLimitX96 *big.Int
}

type UniswapPrice struct {
	TokenPrice *big.Int
	PoolFee    []byte
}

func ethCallUnpack(ctx context.Context, client *ethclient.Client, to common.Address, method string, args ...any) ([]any, error) {
	data, err := uniswapQuoteABI.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	return uniswapQuoteABI.Unpack(method, out)
}

// searchPoolAndFee finds the Uniswap V3 fee tier whose pool holds the most token1, mirroring the
// TS searchPoolAndFee liquidity heuristic.
func searchPoolAndFee(ctx context.Context, client *ethclient.Client, factory, token0, token1 common.Address) (*big.Int, error) {
	bestFee := big.NewInt(0)
	maxBalance := big.NewInt(0)
	for _, fee := range uniswapPoolFees {
		feeBig := big.NewInt(fee)
		poolValues, err := ethCallUnpack(ctx, client, factory, "getPool", token0, token1, feeBig)
		if err != nil {
			return nil, err
		}
		pool, ok := poolValues[0].(common.Address)
		if !ok || pool == (common.Address{}) {
			continue
		}
		balanceValues, err := ethCallUnpack(ctx, client, token1, "balanceOf", pool)
		if err != nil {
			return nil, err
		}
		balance, ok := balanceValues[0].(*big.Int)
		if !ok {
			continue
		}
		if balance.Cmp(maxBalance) >= 0 {
			maxBalance = balance
			bestFee = feeBig
		}
	}
	if maxBalance.Sign() == 0 {
		return nil, errUniswapNoLiquidity
	}
	return bestFee, nil
}

func getUniswapPriceHelper(ctx context.Context, client *ethclient.Client, quoter, tokenIn, tokenOut common.Address, fee, amountIn *big.Int) (*big.Int, error) {
	values, err := ethCallUnpack(ctx, client, quoter, "quoteExactInputSingle", quoteExactInputSingleParams{
		TokenIn:           tokenIn,
		TokenOut:          tokenOut,
		AmountIn:          amountIn,
		Fee:               fee,
		SqrtPriceLimitX96: big.NewInt(0),
	})
	if err != nil {
		return nil, err
	}
	amountOut, ok := values[0].(*big.Int)
	if !ok {
		return nil, errNoUniswapPrice
	}
	return amountOut, nil
}

func GetUniswapPrice(
	ctx context.Context,
	chainID int,
	inSwapAmount string,
	inSwapToken, outSwapToken types.ERC20Token,
) (UniswapPrice, error) {
	addresses, err := constants.GetUniswapV3Addresses(chainID)
	if err != nil {
		return UniswapPrice{}, err
	}
	rpcURL, err := constants.FetchRPCURL(chainID)
	if err != nil {
		return UniswapPrice{}, err
	}
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return UniswapPrice{}, err
	}
	defer client.Close()

	factory := common.HexToAddress(addresses.UniswapV3FactoryAddress)
	quoter := common.HexToAddress(addresses.QuoterV2Address)
	tokenIn := common.HexToAddress(inSwapToken.Erc20TokenAddress)
	tokenOut := common.HexToAddress(outSwapToken.Erc20TokenAddress)

	fee, err := searchPoolAndFee(ctx, client, factory, tokenIn, tokenOut)
	if err != nil {
		return UniswapPrice{}, err
	}
	poolFee, err := uint24Argument.Pack(fee)
	if err != nil {
		return UniswapPrice{}, err
	}

	inSwapAmountWei, err := GetAmountInWei(inSwapToken, inSwapAmount)
	if err != nil {
		return UniswapPrice{}, err
	}
	tokenPrice, err := getUniswapPriceHelper(ctx, client, quoter, tokenIn, tokenOut, fee, inSwapAmountWei)
	if err != nil {
		return UniswapPrice{}, err
	}
	return UniswapPrice{TokenPrice: tokenPrice, PoolFee: poolFee}, nil
}
