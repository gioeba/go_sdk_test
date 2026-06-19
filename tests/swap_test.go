package tests

import (
	"context"
	"encoding/hex"
	"math/big"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
)

const (
	optimismUSDC = "0x0b2C639c533813f4Aa9D7837CAf62653d097Ff85"
	optimismUSDT = "0x94b008aa00579c1307b0ef2c499ad98a8ce58e58"

	evmSwapSlippagePercent = 0.7
)

type evmSwapQuote struct {
	outAmount        *big.Int
	swapData         string
	externalActionID types.ExternalActionID
}

type evmSwapQuoteProvider struct {
	name  string
	quote func(context.Context, *ethclient.Client, int, string, types.ERC20Token, types.ERC20Token) (evmSwapQuote, error)
}

func swapEnv(t *testing.T) (chainID int, inToken, outToken, amount string) {
	t.Helper()
	chainID = constants.ChainIDs.Optimism
	if v := os.Getenv("HINKAL_SWAP_CHAIN"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("bad HINKAL_SWAP_CHAIN: %q", v)
		}
		chainID = parsed
	}
	inToken = envOr("HINKAL_SWAP_IN_TOKEN", optimismUSDC)
	outToken = envOr("HINKAL_SWAP_OUT_TOKEN", optimismUSDT)
	amount = envOr("HINKAL_SWAP_AMOUNT", "0.1")
	return chainID, inToken, outToken, amount
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func evmSwapQuoteProviders() []evmSwapQuoteProvider {
	return []evmSwapQuoteProvider{
		{
			name: "uniswap",
			quote: func(ctx context.Context, _ *ethclient.Client, chainID int, amount string, inToken, outToken types.ERC20Token) (evmSwapQuote, error) {
				quote, err := web3.GetUniswapPrice(ctx, chainID, amount, inToken, outToken)
				if err != nil {
					return evmSwapQuote{}, err
				}
				return evmSwapQuote{
					outAmount:        quote.TokenPrice,
					swapData:         "0x" + hex.EncodeToString(quote.PoolFee),
					externalActionID: types.ExternalActionUniswap,
				}, nil
			},
		},
		{
			name: "odos",
			quote: func(ctx context.Context, _ *ethclient.Client, chainID int, amount string, inToken, outToken types.ERC20Token) (evmSwapQuote, error) {
				quote, err := web3.GetOdosPrice(ctx, chainID, inToken, outToken, amount, evmSwapSlippagePercent)
				if err != nil {
					return evmSwapQuote{}, err
				}
				return evmSwapQuote{
					outAmount:        quote.OutSwapAmount,
					swapData:         quote.OdosData,
					externalActionID: types.ExternalActionOdos,
				}, nil
			},
		},
		{
			name: "oneinch",
			quote: func(ctx context.Context, _ *ethclient.Client, chainID int, amount string, inToken, outToken types.ERC20Token) (evmSwapQuote, error) {
				quote, err := web3.GetOneInchPrice(ctx, chainID, inToken, outToken, amount, evmSwapSlippagePercent)
				if err != nil {
					return evmSwapQuote{}, err
				}
				return evmSwapQuote{
					outAmount:        quote.OutSwapAmount,
					swapData:         quote.OneInchData,
					externalActionID: types.ExternalActionOneInch,
				}, nil
			},
		},
	}
}

func runEVMSwapProviderLive(
	t *testing.T,
	ctx context.Context,
	h *hinkal.Hinkal,
	client *ethclient.Client,
	provider evmSwapQuoteProvider,
	chainID int,
	ethAddress string,
	inTokenAddr string,
	outTokenAddr string,
	amount string,
) {
	t.Helper()

	tokens := web3.ResolveERC20Tokens(chainID, []string{inTokenAddr, outTokenAddr})
	inToken, outToken := tokens[0], tokens[1]
	t.Logf("swap config: provider=%s chain=%d amount=%s in=%s(%d) out=%s(%d)", provider.name, chainID, amount, inToken.Symbol, inToken.Decimals, outToken.Symbol, outToken.Decimals)

	inAmountWei, err := web3.GetAmountInWei(inToken, amount)
	if err != nil {
		t.Fatalf("in amount: %v", err)
	}

	quote, err := provider.quote(ctx, client, chainID, amount, inToken, outToken)
	if err != nil {
		t.Fatalf("%s quote: %v", provider.name, err)
	}
	t.Logf("%s quote: in=%s out=%s", provider.name, inAmountWei, quote.outAmount)

	_, depositTxHash, err := h.Deposit(ctx, chainID, []string{inTokenAddr}, []*big.Int{inAmountWei}, true, false)
	if err != nil {
		t.Fatalf("deposit before swap: %v", err)
	}
	if _, err := h.WaitForTransaction(ctx, chainID, depositTxHash, 1); err != nil {
		t.Fatalf("wait for deposit tx: %v", err)
	}
	time.Sleep(10 * time.Second)

	outBefore := privateBalanceForToken(t, ctx, h, chainID, ethAddress, outTokenAddr)
	deltaAmounts := []*big.Int{new(big.Int).Neg(inAmountWei), quote.outAmount}
	swapTxHash, err := h.Swap(ctx, chainID, []string{inTokenAddr, outTokenAddr}, deltaAmounts, quote.externalActionID, quote.swapData, inTokenAddr, nil)
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	t.Logf("swap tx: %s", swapTxHash)
	if _, err := h.WaitForTransaction(ctx, chainID, swapTxHash, 1); err != nil {
		t.Fatalf("wait for swap tx: %v", err)
	}
	time.Sleep(10 * time.Second)

	outAfter := privateBalanceForToken(t, ctx, h, chainID, ethAddress, outTokenAddr)
	delta := new(big.Int).Sub(outAfter, outBefore)
	t.Logf("private %s after swap: before=%s after=%s delta=%s", outTokenAddr, outBefore, outAfter, delta)
	if delta.Sign() <= 0 {
		t.Fatalf("expected output token private balance to increase, got delta=%s", delta)
	}
}

// HINKAL_LIVE=1 HINKAL_PRIVATE_KEY=0x... go test ./tests/ -run TestSwap_Live -v
// (defaults to Optimism 0.1 USDC -> USDT; override with HINKAL_SWAP_CHAIN / _IN_TOKEN / _OUT_TOKEN / _AMOUNT)
func TestSwap_Live(t *testing.T) {
	requireLive(t)
	chainID, inTokenAddr, outTokenAddr, amount := swapEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	h, ethAddress := newLiveEVMHinkal(t, ctx, chainID)
	client, err := h.GetFetchClient(chainID)
	if err != nil {
		t.Fatalf("fetch client: %v", err)
	}

	for _, provider := range evmSwapQuoteProviders() {
		provider := provider
		t.Run(provider.name, func(t *testing.T) {
			runEVMSwapProviderLive(t, ctx, h, client, provider, chainID, ethAddress, inTokenAddr, outTokenAddr, amount)
		})
	}
}
