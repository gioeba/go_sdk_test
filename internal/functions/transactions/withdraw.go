package transactions

import (
	"context"
	"errors"
	"math/big"

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
)

var (
	errWithdrawNoToken     = errors.New("transactions: withdraw action: no token found")
	errRelayerNotAvailable = errors.New("RELAYER_NOT_AVAILABLE")
)

func copyBigInts(values []*big.Int) []*big.Int {
	out := make([]*big.Int, len(values))
	for i, value := range values {
		out[i] = new(big.Int).Set(value)
	}
	return out
}

func tokenAddresses(tokens []types.ERC20Token) []string {
	addresses := make([]string, len(tokens))
	for i, token := range tokens {
		addresses[i] = token.Erc20TokenAddress
	}
	return addresses
}

func resolveWithdrawFeeStructure(
	ctx context.Context,
	chainID int,
	feeToken string,
	erc20Addresses []string,
	token types.ERC20Token,
	amount *big.Int,
	feeStructureOverride *types.FeeStructure,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (types.FeeStructure, error) {
	var rawFeeStructure types.FeeStructure
	if feeStructureOverride != nil {
		rawFeeStructure = *feeStructureOverride
	} else {
		var err error
		rawFeeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, feeToken, erc20Addresses, types.ExternalActionTransact, nil, nil, solanaTransactionParams)
		if err != nil {
			return types.FeeStructure{}, err
		}
	}
	return fees.CalculateModifiedFeeStructure(ctx, chainID, token, new(big.Int).Neg(amount), rawFeeStructure), nil
}

func relayerAddress(ctx context.Context, hinkal ihinkal.HinkalInternal, chainID int) (string, error) {
	relay, err := hinkal.GetRandomRelay(ctx, chainID, true)
	if err != nil {
		return "", err
	}
	if relay == "" {
		return "", errRelayerNotAvailable
	}
	return relay, nil
}

func buildWithdrawProof(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	recipientAddress string,
	feeStructure types.FeeStructure,
	relay string,
	originalSender string,
) (snarkjs.ConstructZkProofResult, error) {
	externalAddress, err := utils.AddressToHexFormat(recipientAddress)
	if err != nil {
		return snarkjs.ConstructZkProofResult{}, err
	}
	inputUtxosArray, outputUtxosArray, err := getInputAndOutputUtxos(ctx, hinkal, chainID, erc20Addresses, amountChanges)
	if err != nil {
		return snarkjs.ConstructZkProofResult{}, err
	}
	return snarkjs.ConstructZkProof(ctx, snarkjs.ConstructZkProofParams{
		MerkleTree:             hinkal.MerkleTree(chainID),
		InputUtxos:             inputUtxosArray,
		OutputUtxos:            outputUtxosArray,
		UserKeys:               hinkal.GetUserKeys(),
		ExternalActionID:       types.ExternalActionZero,
		ExternalAddress:        externalAddress,
		ExternalActionMetadata: nil,
		GenerateProofRemotely:  hinkal.GenerateProofRemotely(),
		FeeStructure:           feeStructure,
		Relay:                  relay,
		ChainID:                chainID,
		OriginalSender:         originalSender,
	})
}

func HinkalWithdraw(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChangesBase []*big.Int,
	recipientAddress string,
	isRelayerOff bool,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (types.TransactionRequest, string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if len(erc20Tokens) != len(amountChangesBase) {
		return types.TransactionRequest{}, "", errTokenAmountLengthMismatch
	}
	if len(erc20Tokens) == 0 {
		return types.TransactionRequest{}, "", errWithdrawNoToken
	}

	amountChanges := pretransaction.ModifyVolatileTokenAmountChanges(ctx, chainID, erc20Tokens, copyBigInts(amountChangesBase), "")
	erc20Addresses := tokenAddresses(erc20Tokens)
	token := erc20Tokens[0]

	feeStructure := types.ZeroFeeStructure()
	if !isRelayerOff {
		feeStructure, err = resolveWithdrawFeeStructure(ctx, chainID, feeToken, erc20Addresses, token, amountChanges[0], feeStructureOverride, nil)
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
		if err := pretransaction.MergeWithFeeStructure(chainID, &erc20Addresses, &amountChanges, feeStructure); err != nil {
			return types.TransactionRequest{}, "", err
		}
	}

	ethereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	originalSender := ""
	if isRelayerOff {
		originalSender = ethereumAddress
		if constants.IsTronLike(chainID) {
			originalSender, err = utils.AddressToHexFormat(ethereumAddress)
			if err != nil {
				return types.TransactionRequest{}, "", err
			}
		}
	}

	relay := constants.ZeroAddress
	if !isRelayerOff {
		relay, err = relayerAddress(ctx, hinkal, chainID)
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
	}
	proof, err := buildWithdrawProof(ctx, hinkal, chainID, erc20Addresses, amountChanges, recipientAddress, feeStructure, relay, originalSender)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	if isRelayerOff {
		directAmounts := []*big.Int{amountChanges[0]}
		directTokens := []types.ERC20Token{token}
		if constants.IsTronLike(chainID) {
			client, err := hinkal.GetTronWeb()
			if err != nil {
				return types.TransactionRequest{}, "", err
			}
			txid, err := tron.TransactCallDirectTron(ctx, client, chainID, tron.TransactCallDirectTronParams{
				Amounts:         directAmounts,
				TokensToApprove: directTokens,
				ZkCallData:      proof.ZkCallData,
				CircomData:      proof.CircomData,
				DimData:         proof.DimData,
				PreEstimateGas:  true,
			})
			return types.TransactionRequest{}, txid, err
		}

		adapter, err := hinkal.GetProviderAdapter(&chainID)
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
		return web3.TransactCallDirect(ctx, adapter, chainID, web3.TransactCallDirectParams{
			Amounts:         directAmounts,
			TokensToApprove: directTokens,
			ZkCallData:      proof.ZkCallData,
			CircomData:      proof.CircomData,
			DimData:         proof.DimData,
			PreEstimateGas:  true,
			ReturnTxData:    false,
		})
	}

	var tronProofSignature *api.TronProofSignature
	if constants.IsTronLike(chainID) {
		signature, err := tron.ReorderZkCallData(ctx, &proof.ZkCallData, proof.DimData, proof.CircomData, true)
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
		tronProofSignature = &signature
	}
	txHash, err := web3.TransactCallRelayer(ctx, chainID, proof.ZkCallData, proof.DimData, proof.CircomData, proof.CommitmentValidationData, false, tronProofSignature)
	return types.TransactionRequest{}, txHash, err
}
