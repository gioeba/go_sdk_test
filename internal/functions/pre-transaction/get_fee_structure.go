package pretransaction

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

func GetFeeStructure(
	ctx context.Context,
	chainID int,
	feeTokenAddress string,
	erc20Addresses []string,
	externalActionID types.ExternalActionID,
	calls []types.CallInfo,
	variableRate *big.Int,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (types.FeeStructure, error) {
	lookupAddress := feeTokenAddress
	if lookupAddress == "" {
		lookupAddress = constants.DefaultFeeToken
	}
	feeToken := constants.GetERC20Token(lookupAddress, chainID)
	if feeToken == nil {
		return types.FeeStructure{}, errors.New("failed to find feeToken")
	}

	numberOfInputs := len(erc20Addresses)
	for _, tokenAddress := range erc20Addresses {
		if strings.EqualFold(tokenAddress, feeTokenAddress) {
			numberOfInputs = len(erc20Addresses) + 1
			break
		}
	}

	_, priceOfTransactionInToken, err := ProcessGasEstimates(ctx, chainID, *feeToken, externalActionID, numberOfInputs, nil, calls, solanaTransactionParams)
	if err != nil {
		return types.FeeStructure{}, err
	}
	if priceOfTransactionInToken == nil {
		return types.FeeStructure{}, errors.New("failed to process gas estimates")
	}

	if variableRate == nil {
		variableRate = big.NewInt(0)
	}
	return types.FeeStructure{
		FeeToken:     feeToken.Erc20TokenAddress,
		FlatFee:      priceOfTransactionInToken,
		VariableRate: variableRate,
	}, nil
}
