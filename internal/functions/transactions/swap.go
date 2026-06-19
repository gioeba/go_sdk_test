package transactions

import (
	"context"
	"math/big"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
)

func swapOnChainCreation(length int) []bool {
	pattern := []bool{false, true, false}
	if length > len(pattern) {
		length = len(pattern)
	}
	return pattern[:length]
}

func HinkalSwap(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	deltaAmountsBase []*big.Int,
	externalActionID types.ExternalActionID,
	data string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	deltaAmounts := copyBigInts(deltaAmountsBase)
	erc20Addresses := tokenAddresses(erc20Tokens)

	var feeStructure types.FeeStructure
	if feeStructureOverride != nil {
		feeStructure = *feeStructureOverride
	} else {
		feeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, feeToken, erc20Addresses, externalActionID, nil, big.NewInt(constants.HinkalSwapVariableRate), nil)
		if err != nil {
			return "", err
		}
	}
	if err := pretransaction.MergeWithFeeStructure(chainID, &erc20Addresses, &deltaAmounts, feeStructure); err != nil {
		return "", err
	}

	externalAddress, err := pretransaction.GetExternalSwapAddress(chainID, externalActionID)
	if err != nil {
		return "", err
	}

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	inputUtxosArray, outputUtxosArray, err := getInputAndOutputUtxos(ctx, hinkal, chainID, erc20Addresses, deltaAmounts)
	if err != nil {
		return "", err
	}

	proof, err := snarkjs.ConstructZkProof(ctx, snarkjs.ConstructZkProofParams{
		MerkleTree:             hinkal.MerkleTree(chainID),
		InputUtxos:             inputUtxosArray,
		OutputUtxos:            outputUtxosArray,
		UserKeys:               hinkal.GetUserKeys(),
		ExternalActionID:       externalActionID,
		ExternalAddress:        externalAddress,
		ExternalActionMetadata: []string{data},
		GenerateProofRemotely:  hinkal.GenerateProofRemotely(),
		FeeStructure:           feeStructure,
		Relay:                  relay,
		ChainID:                chainID,
		OnChainCreation:        swapOnChainCreation(len(deltaAmounts)),
	})
	if err != nil {
		return "", err
	}

	return web3.TransactCallRelayer(ctx, chainID, proof.ZkCallData, proof.DimData, proof.CircomData, proof.CommitmentValidationData, false, nil)
}
