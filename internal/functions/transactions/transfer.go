package transactions

import (
	"context"
	"errors"
	"math/big"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	errorhandling "github.com/gioeba/go_sdk_test/error-handling"
	"github.com/gioeba/go_sdk_test/internal/functions/balance"
	"github.com/gioeba/go_sdk_test/internal/functions/fees"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	"github.com/gioeba/go_sdk_test/internal/functions/tron"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

var errTransferNoToken = errors.New("transactions: transfer action: no token found")

func privateSendFeeStructure(feeStructure types.FeeStructure) types.FeeStructure {
	if feeStructure.FeeToken == "" {
		feeStructure.FeeToken = constants.DefaultFeeToken
	}
	if feeStructure.FlatFee == nil {
		feeStructure.FlatFee = big.NewInt(0)
	}
	if feeStructure.VariableRate == nil || feeStructure.VariableRate.Sign() == 0 {
		feeStructure.VariableRate = big.NewInt(constants.HinkalPrivateSendVariableRate)
	}
	return feeStructure
}

func resolveTransferFeeStructure(
	ctx context.Context,
	chainID int,
	feeToken string,
	erc20Addresses []string,
	sentToken types.ERC20Token,
	recipientAmount *big.Int,
	feeStructureOverride *types.FeeStructure,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (types.FeeStructure, error) {
	var rawFeeStructure types.FeeStructure
	if feeStructureOverride != nil {
		rawFeeStructure = *feeStructureOverride
	} else {
		var err error
		rawFeeStructure, err = pretransaction.GetFeeStructure(
			ctx,
			chainID,
			feeToken,
			erc20Addresses,
			types.ExternalActionTransact,
			nil,
			big.NewInt(constants.HinkalPrivateSendVariableRate),
			solanaTransactionParams,
		)
		if err != nil {
			return types.FeeStructure{}, err
		}
	}
	return fees.CalculateModifiedFeeStructure(ctx, chainID, sentToken, recipientAmount, privateSendFeeStructure(rawFeeStructure)), nil
}

func getTransferInputAndOutputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	recipientAddress string,
	recipientAmount *big.Int,
) (inputUtxosArray, outputUtxosArray [][]*utxo.Utxo, err error) {
	inputUtxosArray, err = balance.AddPaddingToUtxos(ctx, hinkal, chainID, erc20Addresses, amountChanges, 0, nil, false, false)
	if err != nil {
		return nil, nil, err
	}

	userKeys := hinkal.GetUserKeys()
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	outputUtxosArray = make([][]*utxo.Utxo, 0, len(erc20Addresses))
	for i := range erc20Addresses {
		var outputRecipientAmount *big.Int
		if i == 0 {
			outputRecipientAmount = recipientAmount
		}
		outputUtxos, err := pretransaction.OutputUtxoProcessing(
			userKeys,
			inputUtxosArray[i],
			amountChanges[i],
			timeStamp,
			true,
			recipientAddress,
			outputRecipientAmount,
		)
		if err != nil {
			return nil, nil, err
		}
		outputUtxosArray = append(outputUtxosArray, outputUtxos)
	}
	return inputUtxosArray, outputUtxosArray, nil
}

func buildTransferProof(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	recipientAddress string,
	recipientAmount *big.Int,
	feeStructure types.FeeStructure,
	relay string,
) (snarkjs.ConstructZkProofResult, error) {
	inputUtxosArray, outputUtxosArray, err := getTransferInputAndOutputUtxos(
		ctx,
		hinkal,
		chainID,
		erc20Addresses,
		amountChanges,
		recipientAddress,
		recipientAmount,
	)
	if err != nil {
		return snarkjs.ConstructZkProofResult{}, err
	}

	return snarkjs.ConstructZkProof(ctx, snarkjs.ConstructZkProofParams{
		MerkleTree:             hinkal.MerkleTree(chainID),
		InputUtxos:             inputUtxosArray,
		OutputUtxos:            outputUtxosArray,
		UserKeys:               hinkal.GetUserKeys(),
		ExternalActionID:       types.ExternalActionZero,
		ExternalAddress:        relay,
		ExternalActionMetadata: nil,
		GenerateProofRemotely:  hinkal.GenerateProofRemotely(),
		FeeStructure:           feeStructure,
		Relay:                  relay,
		ChainID:                chainID,
	})
}

func HinkalTransfer(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChangesBase []*big.Int,
	recipientAddress string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	if len(erc20Tokens) != len(amountChangesBase) {
		return "", errTokenAmountLengthMismatch
	}
	if len(erc20Tokens) == 0 {
		return "", errTransferNoToken
	}
	if !pretransaction.IsValidPrivateAddress(recipientAddress) {
		return "", errorhandling.ErrRecipientFormatIncorrect
	}

	amountChanges := pretransaction.ModifyVolatileTokenAmountChanges(ctx, chainID, erc20Tokens, copyBigInts(amountChangesBase), "")
	erc20Addresses := tokenAddresses(erc20Tokens)
	sentToken := erc20Tokens[0]
	recipientAmount := new(big.Int).Neg(amountChanges[0])

	feeStructure, err := resolveTransferFeeStructure(ctx, chainID, feeToken, erc20Addresses, sentToken, recipientAmount, feeStructureOverride, nil)
	if err != nil {
		return "", err
	}
	if err := pretransaction.MergeWithFeeStructure(chainID, &erc20Addresses, &amountChanges, feeStructure); err != nil {
		return "", err
	}

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	proof, err := buildTransferProof(ctx, hinkal, chainID, erc20Addresses, amountChanges, recipientAddress, recipientAmount, feeStructure, relay)
	if err != nil {
		return "", err
	}

	var tronProofSignature *api.TronProofSignature
	if constants.IsTronLike(chainID) {
		signature, err := tron.ReorderZkCallData(ctx, &proof.ZkCallData, proof.DimData, proof.CircomData, true)
		if err != nil {
			return "", err
		}
		tronProofSignature = &signature
	}
	return web3.TransactCallRelayer(ctx, chainID, proof.ZkCallData, proof.DimData, proof.CircomData, proof.CommitmentValidationData, false, tronProofSignature)
}
