package transactions

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	"github.com/gioeba/go_sdk_test/internal/functions/fees"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	"github.com/gioeba/go_sdk_test/internal/functions/tron"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

var (
	errDepositAndWithdrawNoToken           = errors.New("transactions: depositAndWithdraw action: no token found")
	errDepositAndWithdrawOneToken          = errors.New("transactions: depositAndWithdraw supports one token")
	errRecipientAmountLengthMismatch       = errors.New("transactions: recipientAmounts and recipientAddresses length mismatch")
	errNoDepositedOnChainUtxos             = errors.New("transactions: no on-chain UTXOs found in deposit transaction")
	errDepositedUtxosMissingFromMerkle     = errors.New("transactions: timeout while waiting for deposited UTXOs to appear in Merkle tree")
	errDepositAndWithdrawUnsupportedSolana = errors.New("transactions: use HinkalSolanaDepositAndWithdraw for Solana chains")
)

func paySendFeeStructure(feeStructure types.FeeStructure) types.FeeStructure {
	if feeStructure.FeeToken == "" {
		feeStructure.FeeToken = constants.DefaultFeeToken
	}
	if feeStructure.FlatFee == nil {
		feeStructure.FlatFee = big.NewInt(0)
	}
	if feeStructure.VariableRate == nil || feeStructure.VariableRate.Sign() == 0 {
		feeStructure.VariableRate = big.NewInt(constants.PaySendVariableRate)
	}
	return feeStructure
}

func resolveDepositAndWithdrawFeeStructure(
	ctx context.Context,
	chainID int,
	tokenAddress string,
	feeStructureOverride *types.FeeStructure,
) (types.FeeStructure, error) {
	if feeStructureOverride != nil {
		return paySendFeeStructure(*feeStructureOverride), nil
	}
	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		tokenAddress,
		[]string{tokenAddress},
		types.ExternalActionTransact,
		nil,
		big.NewInt(constants.PaySendVariableRate),
		nil,
	)
	if err != nil {
		return types.FeeStructure{}, err
	}
	return paySendFeeStructure(feeStructure), nil
}

func validateDepositAndWithdrawArgs(
	erc20Tokens []types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
) error {
	if len(erc20Tokens) == 0 {
		return errDepositAndWithdrawNoToken
	}
	if len(erc20Tokens) > 1 {
		return errDepositAndWithdrawOneToken
	}
	if len(recipientAmounts) == 0 {
		return errAmountsEmpty
	}
	if len(recipientAmounts) != len(recipientAddresses) {
		return errRecipientAmountLengthMismatch
	}
	for _, amount := range recipientAmounts {
		if amount == nil || amount.Sign() <= 0 {
			return errAmountNotPositive
		}
	}
	return nil
}

func areAllDepositedUtxosInLastLeaves(deposited []recipientUtxo, lastLeaves []*big.Int) (bool, error) {
	if len(deposited) == 0 {
		return true, nil
	}
	leafSet := make(map[string]struct{}, len(lastLeaves))
	for _, leaf := range lastLeaves {
		leafSet[leaf.String()] = struct{}{}
	}
	for _, depositedUtxo := range deposited {
		commitment, err := depositedUtxo.utxo.GetCommitment()
		if err != nil {
			return false, err
		}
		commitmentBig, err := utils.ParseBigInt(commitment)
		if err != nil {
			return false, err
		}
		if _, ok := leafSet[commitmentBig.String()]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func waitForDepositedUtxosInMerkleTree(ctx context.Context, hinkal ihinkal.HinkalInternal, chainID int, deposited []recipientUtxo) error {
	if len(deposited) == 0 {
		return nil
	}
	for attempt := 0; attempt < 60; attempt++ {
		if err := hinkal.ResetMerkle(ctx, chainID); err != nil {
			return err
		}
		var lastLeaves []*big.Int
		if tree := hinkal.MerkleTree(chainID); tree != nil {
			lastLeaves = tree.LastLeaves(20)
		}
		found, err := areAllDepositedUtxosInLastLeaves(deposited, lastLeaves)
		if err != nil {
			return err
		}
		if found {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errDepositedUtxosMissingFromMerkle
}

func buildDepositAndWithdrawZeroUtxo(
	hinkal ihinkal.HinkalInternal,
	tokenAddress string,
	source *utxo.Utxo,
	timeStamp string,
) (*utxo.Utxo, error) {
	shieldedPrivateKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	params := types.UtxoParams{
		Amount:            big.NewInt(0),
		Erc20TokenAddress: tokenAddress,
		NullifyingKey:     shieldedPrivateKey,
		TimeStamp:         timeStamp,
		IsNewStyle:        source.IsNewStyle,
	}
	if source.IsNewStyle {
		spendingKeyPair, err := hinkal.GetUserKeys().GetSpendingKeyPair()
		if err != nil {
			return nil, err
		}
		params.SpendingPublicKey = []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}
	}
	return utxo.NewUtxo(params)
}

func hinkalWithdrawBatch(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	userDepositedUtxos []recipientUtxo,
	recipientAmounts []*big.Int,
	feeStructure types.FeeStructure,
	hashedEthereumAddress string,
	statusID string,
	txCompletionTime *int,
) (string, error) {
	if len(userDepositedUtxos) == 0 {
		return "", errNoDepositedOnChainUtxos
	}
	tokenAddress := erc20Token.Erc20TokenAddress
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	withdrawTimeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	transactions := make([]web3.TransactCallRelayerBatchItem, 0, len(userDepositedUtxos))

	for i, item := range userDepositedUtxos {
		recipientAddressHex, err := utils.AddressToHexFormat(item.recipientAddress)
		if err != nil {
			return "", err
		}
		zeroUtxo, err := buildDepositAndWithdrawZeroUtxo(hinkal, tokenAddress, item.utxo, withdrawTimeStamp)
		if err != nil {
			return "", err
		}
		utxoToWithdraw := item.utxo
		if !strings.EqualFold(utxoToWithdraw.Erc20TokenAddress, tokenAddress) {
			utxoToWithdraw, err = utxo.CreateFrom(utxoToWithdraw, types.UtxoParams{Erc20TokenAddress: tokenAddress})
			if err != nil {
				return "", err
			}
		}

		withdrawInputUtxosArray := [][]*utxo.Utxo{{utxoToWithdraw, zeroUtxo}}
		withdrawOutputUtxosArray := [][]*utxo.Utxo{{zeroUtxo}}
		withdrawFeeStructure := fees.CalculateModifiedFeeStructure(ctx, chainID, erc20Token, recipientAmounts[i], feeStructure)
		proof, err := snarkjs.ConstructZkProof(ctx, snarkjs.ConstructZkProofParams{
			MerkleTree:             hinkal.MerkleTree(chainID),
			InputUtxos:             withdrawInputUtxosArray,
			OutputUtxos:            withdrawOutputUtxosArray,
			UserKeys:               hinkal.GetUserKeys(),
			ExternalActionID:       types.ExternalActionZero,
			ExternalAddress:        recipientAddressHex,
			ExternalActionMetadata: nil,
			GenerateProofRemotely:  hinkal.GenerateProofRemotely(),
			FeeStructure:           withdrawFeeStructure,
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

		transactions = append(transactions, web3.TransactCallRelayerBatchItem{
			ZkCallData:               proof.ZkCallData,
			DimData:                  proof.DimData,
			CircomData:               proof.CircomData,
			CommitmentValidationData: proof.CommitmentValidationData,
			RecipientAddress:         item.recipientAddress,
			TronProofSignature:       tronProofSignature,
		})
	}

	_, _ = api.SafeUpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ID:                    statusID,
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseBeforeScheduleWithdraw,
	})
	scheduleID, err := web3.TransactCallRelayerBatch(ctx, chainID, transactions, hashedEthereumAddress, txCompletionTime, "", "")
	if err != nil {
		return "", err
	}
	_, _ = api.SafeUpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ID:                    statusID,
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseAfterScheduleWithdraw,
		ScheduleID:            scheduleID,
	})
	return scheduleID, nil
}

func HinkalDepositAndWithdraw(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
	preEstimateGas bool,
) (types.DepositAndSendExtendedResult, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if constants.IsSolanaLike(chainID) {
		return types.DepositAndSendExtendedResult{}, errDepositAndWithdrawUnsupportedSolana
	}
	if err := validateDepositAndWithdrawArgs(erc20Tokens, recipientAmounts, recipientAddresses); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	erc20Token := erc20Tokens[0]
	tokenAddress := erc20Token.Erc20TokenAddress
	rawEthereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	ethereumAddress, err := utils.AddressToHexFormat(rawEthereumAddress)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	hashedEthereumAddress := utils.HashEthereumAddress(ethereumAddress)

	feeStructure, err := resolveDepositAndWithdrawFeeStructure(ctx, chainID, tokenAddress, feeStructureOverride)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	userDepositedUtxos, statusID, depositTxHash, err := HinkalDepositOnChainUtxos(
		ctx,
		hinkal,
		chainID,
		erc20Token,
		recipientAmounts,
		recipientAddresses,
		feeStructure,
		hashedEthereumAddress,
		preEstimateGas,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if err := waitForDepositedUtxosInMerkleTree(ctx, hinkal, chainID, userDepositedUtxos); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	scheduleID, err := hinkalWithdrawBatch(
		ctx,
		hinkal,
		chainID,
		erc20Token,
		userDepositedUtxos,
		recipientAmounts,
		feeStructure,
		hashedEthereumAddress,
		statusID,
		txCompletionTime,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	return types.DepositAndSendExtendedResult{DepositTxHash: depositTxHash, ScheduleID: scheduleID}, nil
}
