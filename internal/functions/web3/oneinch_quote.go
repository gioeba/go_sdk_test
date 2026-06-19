package web3

import (
	"context"
	"errors"
	"math/big"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

var errNoOneInchPrice = errors.New("web3: failed to fetch 1inch price")

type OneInchPrice struct {
	OutSwapAmount *big.Int
	OneInchData   string
}

func GetOneInchPrice(
	ctx context.Context,
	chainID int,
	inSwapToken, outSwapToken types.ERC20Token,
	inSwapAmount string,
	slippagePercentage float64,
) (OneInchPrice, error) {
	inSwapAmountWei, err := GetAmountInWei(inSwapToken, inSwapAmount)
	if err != nil {
		return OneInchPrice{}, err
	}

	contractData, err := constants.GetContractData(chainID)
	if err != nil {
		return OneInchPrice{}, err
	}

	fromToken := inSwapToken.Erc20TokenAddress
	if fromToken == constants.ZeroAddress {
		fromToken = constants.OneInchZeroAddress
	}
	toToken := outSwapToken.Erc20TokenAddress
	if toToken == constants.ZeroAddress {
		toToken = constants.OneInchZeroAddress
	}

	response, err := api.CallOneInchAPI(ctx, chainID, api.OneInchRequest{
		FromTokenAddress: fromToken,
		ToTokenAddress:   toToken,
		FromAddress:      constants.ZeroAddress,
		DestReceiver:     contractData.OneInchExternalActionInstanceAddress,
		Amount:           inSwapAmountWei,
		Slippage:         slippagePercentage,
		DisableEstimate:  true,
		AllowPartialFill: false,
	})
	if err != nil {
		return OneInchPrice{}, err
	}
	if response.Tx.Data == "" {
		return OneInchPrice{}, errNoOneInchPrice
	}
	outSwapAmount, ok := new(big.Int).SetString(response.ToAmount, 10)
	if !ok {
		return OneInchPrice{}, errNoOneInchPrice
	}
	return OneInchPrice{OutSwapAmount: outSwapAmount, OneInchData: response.Tx.Data}, nil
}
