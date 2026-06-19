package integrations

import (
	"context"
	"time"

	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
)

const defaultSwapQuoteTimeout = 30 * time.Second

type EVMSwapPrices struct {
	Uniswap *web3.UniswapPrice `json:"uniswap,omitempty"`
	Odos    *web3.OdosPrice    `json:"odos,omitempty"`
	OneInch *web3.OneInchPrice `json:"oneInch,omitempty"`
}

type SolanaSwapPrices struct {
	OKX *pretransaction.OKXPrice `json:"okx,omitempty"`
}

type evmQuoteResult struct {
	uniswap *web3.UniswapPrice
	odos    *web3.OdosPrice
	oneInch *web3.OneInchPrice
}

func GetEVMSwapPrices(
	ctx context.Context,
	chainID int,
	inSwapAmount string,
	inSwapTokenAddress string,
	outSwapTokenAddress string,
) (EVMSwapPrices, error) {
	tokens, err := web3.ResolveERC20TokensStrict(ctx, chainID, []string{inSwapTokenAddress, outSwapTokenAddress})
	if err != nil {
		return EVMSwapPrices{}, err
	}
	inSwapToken, outSwapToken := tokens[0], tokens[1]

	quoteCtx, cancel := context.WithTimeout(ctx, defaultSwapQuoteTimeout)
	defer cancel()

	results := make(chan evmQuoteResult, 3)
	go func() {
		price, err := web3.GetUniswapPrice(quoteCtx, chainID, inSwapAmount, inSwapToken, outSwapToken)
		if err != nil {
			results <- evmQuoteResult{}
			return
		}
		results <- evmQuoteResult{uniswap: &price}
	}()
	go func() {
		price, err := web3.GetOdosPrice(quoteCtx, chainID, inSwapToken, outSwapToken, inSwapAmount, 0.7)
		if err != nil {
			results <- evmQuoteResult{}
			return
		}
		results <- evmQuoteResult{odos: &price}
	}()
	go func() {
		price, err := web3.GetOneInchPrice(quoteCtx, chainID, inSwapToken, outSwapToken, inSwapAmount, 0.7)
		if err != nil {
			results <- evmQuoteResult{}
			return
		}
		results <- evmQuoteResult{oneInch: &price}
	}()

	var out EVMSwapPrices
	for i := 0; i < 3; i++ {
		result := <-results
		if result.uniswap != nil {
			out.Uniswap = result.uniswap
		}
		if result.odos != nil {
			out.Odos = result.odos
		}
		if result.oneInch != nil {
			out.OneInch = result.oneInch
		}
	}
	return out, nil
}

func GetSolanaSwapPrices(
	ctx context.Context,
	chainID int,
	inSwapAmount string,
	inSwapTokenAddress string,
	outSwapTokenAddress string,
) (SolanaSwapPrices, error) {
	tokens, err := web3.ResolveERC20TokensStrict(ctx, chainID, []string{inSwapTokenAddress, outSwapTokenAddress})
	if err != nil {
		return SolanaSwapPrices{}, err
	}

	quoteCtx, cancel := context.WithTimeout(ctx, defaultSwapQuoteTimeout)
	defer cancel()

	price, err := pretransaction.GetOKXPrice(quoteCtx, chainID, tokens[0], tokens[1], inSwapAmount, 0.5)
	if err != nil {
		return SolanaSwapPrices{}, nil
	}
	return SolanaSwapPrices{OKX: &price}, nil
}
