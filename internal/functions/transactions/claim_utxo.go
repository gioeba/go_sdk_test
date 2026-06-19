package transactions

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	errorhandling "github.com/gioeba/go_sdk_test/error-handling"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	"github.com/gioeba/go_sdk_test/internal/functions/tron"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

var (
	errClaimNoToken                = errors.New("transactions: claimUtxo action: no token found")
	errClaimTooManyTokens          = errors.New("transactions: claimUtxo supports one token")
	errClaimUtxoMissingKey         = errors.New("Claimable UTXO nullifyingKey is missing")
	errClaimUtxoKeyMismatch        = errors.New("Claimable UTXO key mismatch")
	errClaimUtxoTokenMismatch      = errors.New("Off-chain UTXO token mismatch")
	errClaimFeeTokenMismatch       = errors.New("Claim fee token mismatch: fee must be paid from claimed UTXO token")
	errClaimNewStyleNeedsSignature = errors.New("Claimable new-style UTXO requires claimableSignature")
)

func claimUtxoUserKeys(source *utxo.Utxo, claimableSignature string) (*cryptokeys.UserKeys, string, error) {
	if claimableSignature != "" {
		keys := cryptokeys.NewUserKeys(claimableSignature)
		resolved, err := keys.GetShieldedPrivateKey()
		return keys, resolved, err
	}
	if source.NullifyingKey == "" {
		return nil, "", errClaimUtxoMissingKey
	}
	if source.IsNewStyle {
		return nil, "", errClaimNewStyleNeedsSignature
	}
	keys := cryptokeys.NewUserKeysWithSignatureAndNullifyingKey(source.NullifyingKey, source.NullifyingKey)
	return keys, source.NullifyingKey, nil
}

func claimFeeParts(sourceAmount *big.Int, feeStructure types.FeeStructure) (types.FeeStructure, *big.Int, error) {
	flatFee := feeStructure.FlatFee
	if flatFee == nil {
		flatFee = big.NewInt(0)
	}
	variableRate := feeStructure.VariableRate
	if variableRate == nil || variableRate.Sign() <= 0 {
		variableRate = big.NewInt(constants.HinkalPrivateSendVariableRate)
	}
	if sourceAmount.Cmp(flatFee) <= 0 {
		return types.FeeStructure{}, nil, errors.New(errorhandling.ErrCodeInsufficientFundsToTransact)
	}

	transferableAmount := new(big.Int).Sub(sourceAmount, flatFee)
	variableFee := new(big.Int).Div(new(big.Int).Mul(transferableAmount, variableRate), big.NewInt(10000))
	recipientAmount := new(big.Int).Sub(transferableAmount, variableFee)
	if recipientAmount.Sign() <= 0 {
		return types.FeeStructure{}, nil, errorhandling.ErrRecipientAmountInvalid
	}

	return types.FeeStructure{
		FeeToken:     feeStructure.FeeToken,
		FlatFee:      new(big.Int).Add(flatFee, variableFee),
		VariableRate: big.NewInt(0),
	}, recipientAmount, nil
}

func claimPaddingUtxo(source *utxo.Utxo, keys *cryptokeys.UserKeys, resolvedNullifyingKey string) (*utxo.Utxo, error) {
	params := types.UtxoParams{
		Amount:            big.NewInt(0),
		Erc20TokenAddress: source.Erc20TokenAddress,
		MintAddress:       source.MintAddress,
		NullifyingKey:     resolvedNullifyingKey,
		IsNewStyle:        source.IsNewStyle,
	}
	if source.IsNewStyle {
		spendingKeyPair, err := keys.GetSpendingKeyPair()
		if err != nil {
			return nil, err
		}
		params.SpendingPublicKey = []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}
	}
	return utxo.NewUtxo(params)
}

func HinkalClaimUtxo(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	claimableUtxo *utxo.Utxo,
	feeStructureOverride *types.FeeStructure,
	claimableSignature string,
) (string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	if constants.IsSolanaLike(chainID) {
		return "", errProoflessNotImplemented
	}
	if len(erc20Tokens) == 0 {
		return "", errClaimNoToken
	}
	if len(erc20Tokens) > 1 {
		return "", errClaimTooManyTokens
	}
	if claimableUtxo == nil || claimableUtxo.Amount == nil {
		return "", errClaimUtxoMissingKey
	}

	tokenAddress := erc20Tokens[0].Erc20TokenAddress
	if !strings.EqualFold(tokenAddress, claimableUtxo.Erc20TokenAddress) {
		return "", errClaimUtxoTokenMismatch
	}

	utxoSpecificUserKeys, resolvedNullifyingKey, err := claimUtxoUserKeys(claimableUtxo, claimableSignature)
	if err != nil {
		return "", err
	}
	if claimableUtxo.NullifyingKey != "" && !strings.EqualFold(claimableUtxo.NullifyingKey, resolvedNullifyingKey) {
		return "", errClaimUtxoKeyMismatch
	}

	sourceUtxo, err := utxo.CreateFrom(claimableUtxo, types.UtxoParams{NullifyingKey: resolvedNullifyingKey})
	if err != nil {
		return "", err
	}
	paddingUtxo, err := claimPaddingUtxo(sourceUtxo, utxoSpecificUserKeys, resolvedNullifyingKey)
	if err != nil {
		return "", err
	}

	var feeStructure types.FeeStructure
	if feeStructureOverride != nil {
		feeStructure = *feeStructureOverride
	} else {
		feeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, tokenAddress, []string{tokenAddress}, types.ExternalActionTransact, nil, nil, nil)
		if err != nil {
			return "", err
		}
	}
	if !strings.EqualFold(feeStructure.FeeToken, tokenAddress) {
		return "", errClaimFeeTokenMismatch
	}

	claimFeeStructure, recipientAmount, err := claimFeeParts(sourceUtxo.Amount, feeStructure)
	if err != nil {
		return "", err
	}

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}

	recipientInfo, err := hinkal.GetRecipientInfo()
	if err != nil {
		return "", err
	}
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	amountChange := new(big.Int).Neg(sourceUtxo.Amount)
	inputUtxos := []*utxo.Utxo{sourceUtxo, paddingUtxo}
	outputUtxos, err := pretransaction.OutputUtxoProcessing(
		utxoSpecificUserKeys,
		inputUtxos,
		amountChange,
		timeStamp,
		true,
		recipientInfo,
		recipientAmount,
	)
	if err != nil {
		return "", err
	}

	proof, err := snarkjs.ConstructZkProof(ctx, snarkjs.ConstructZkProofParams{
		MerkleTree:             hinkal.MerkleTree(chainID),
		InputUtxos:             [][]*utxo.Utxo{inputUtxos},
		OutputUtxos:            [][]*utxo.Utxo{outputUtxos},
		UserKeys:               utxoSpecificUserKeys,
		ExternalActionID:       types.ExternalActionZero,
		ExternalAddress:        relay,
		ExternalActionMetadata: nil,
		GenerateProofRemotely:  hinkal.GenerateProofRemotely(),
		FeeStructure:           claimFeeStructure,
		Relay:                  relay,
		ChainID:                chainID,
	})
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
