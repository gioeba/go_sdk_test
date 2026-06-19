package web3

import (
	"context"
	"errors"
	"math/big"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

var errNoOdosPrice = errors.New("web3: failed to fetch Odos price")

type OdosPrice struct {
	OutSwapAmount *big.Int
	OdosData      string
}

func GetOdosPrice(
	ctx context.Context,
	chainID int,
	inSwapToken, outSwapToken types.ERC20Token,
	inSwapAmount string,
	slippagePercentage float64,
) (OdosPrice, error) {
	inSwapAmountWei, err := GetAmountInWei(inSwapToken, inSwapAmount)
	if err != nil {
		return OdosPrice{}, err
	}

	userAddr := constants.ZeroAddress
	if contractData, err := constants.GetContractData(chainID); err == nil && contractData.OdosExternalActionInstanceAddress != "" {
		userAddr = contractData.OdosExternalActionInstanceAddress
	}

	requestChainID := chainID
	if chainID == constants.ChainIDs.Localhost {
		requestChainID = constants.LocalhostNetwork
	}

	request := api.OdosSwapRequest{
		ChainID:              requestChainID,
		InputTokens:          []api.OdosInputToken{{TokenAddress: odosTokenAddress(inSwapToken), Amount: inSwapAmountWei.String()}},
		OutputTokens:         []api.OdosOutputToken{{TokenAddress: odosTokenAddress(outSwapToken), Proportion: 1}},
		UserAddr:             userAddr,
		SlippageLimitPercent: slippagePercentage,
		DisableRFQs:          true,
	}

	odosResponse, status, err := api.CallOdosAPI(ctx, chainID, request)
	if err != nil {
		return OdosPrice{}, err
	}
	if status != "success" || len(odosResponse.OutputTokens) == 0 {
		return OdosPrice{}, errNoOdosPrice
	}

	outSwapAmount, ok := new(big.Int).SetString(odosResponse.OutputTokens[0].Amount.String(), 10)
	if !ok {
		return OdosPrice{}, errNoOdosPrice
	}
	return OdosPrice{OutSwapAmount: outSwapAmount, OdosData: odosResponse.Transaction.Data}, nil
}

func odosTokenAddress(token types.ERC20Token) string {
	if token.WrappedErc20TokenAddress != "" {
		return token.WrappedErc20TokenAddress
	}
	return token.Erc20TokenAddress
}
