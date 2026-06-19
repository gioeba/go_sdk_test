package transactions

import (
	"context"
	"math/big"
	"strings"

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

func recipientInputOutputUtxos(
	userKeys *cryptokeys.UserKeys,
	erc20Addresses []string,
	amountChanges []*big.Int,
	recipientInfo string,
) (inputUtxosArray, outputUtxosArray [][]*utxo.Utxo, err error) {
	parts := strings.Split(recipientInfo, ",")
	stealthAddress, h00, h01, encryptionKey := parts[0], parts[1], parts[2], parts[4]
	h00Big, err := utils.ParseBigInt(h00)
	if err != nil {
		return nil, nil, err
	}
	h01Big, err := utils.ParseBigInt(h01)
	if err != nil {
		return nil, nil, err
	}

	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return nil, nil, err
	}
	spendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}

	inputUtxosArray = make([][]*utxo.Utxo, len(erc20Addresses))
	outputUtxosArray = make([][]*utxo.Utxo, len(erc20Addresses))
	for i, erc20TokenAddress := range erc20Addresses {
		inputs := make([]*utxo.Utxo, 2)
		for j := 0; j < 2; j++ {
			in, err := utxo.NewUtxo(types.UtxoParams{
				Amount:            big.NewInt(0),
				Erc20TokenAddress: erc20TokenAddress,
				NullifyingKey:     shieldedPrivateKey,
				SpendingPublicKey: spendingPublicKey,
				IsNewStyle:        true,
			})
			if err != nil {
				return nil, nil, err
			}
			inputs[j] = in
		}
		inputUtxosArray[i] = inputs

		out, err := utxo.NewUtxo(types.UtxoParams{
			Amount:            amountChanges[i],
			Erc20TokenAddress: erc20TokenAddress,
			H0:                &types.JubPoint{h00Big, h01Big},
			StealthAddress:    stealthAddress,
			EncryptionKey:     encryptionKey,
			IsNewStyle:        true,
		})
		if err != nil {
			return nil, nil, err
		}
		outputUtxosArray[i] = []*utxo.Utxo{out}
	}
	return inputUtxosArray, outputUtxosArray, nil
}

func HinkalDepositForOther(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChanges []*big.Int,
	recipientInfo string,
	preEstimateGas bool,
	returnTxData bool,
) (types.TransactionRequest, string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if constants.IsSolanaLike(chainID) {
		return types.TransactionRequest{}, "", errDepositNotImplemented
	}
	if !pretransaction.IsValidPrivateAddress(recipientInfo) {
		return types.TransactionRequest{}, "", errorhandling.ErrRecipientFormatIncorrect
	}

	erc20Addresses := make([]string, len(erc20Tokens))
	for i, token := range erc20Tokens {
		erc20Addresses[i] = token.Erc20TokenAddress
	}

	inputUtxosArray, outputUtxosArray, err := recipientInputOutputUtxos(hinkal.GetUserKeys(), erc20Addresses, amountChanges, recipientInfo)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	params, err := getTransactionParameters(ctx, hinkal, chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	proof, err := snarkjs.ConstructZkProof(ctx, snarkjs.ConstructZkProofParams{
		MerkleTree:             hinkal.MerkleTree(chainID),
		InputUtxos:             inputUtxosArray,
		OutputUtxos:            outputUtxosArray,
		UserKeys:               hinkal.GetUserKeys(),
		ExternalActionID:       params.externalActionID,
		ExternalAddress:        params.externalAddress,
		ExternalActionMetadata: params.externalActionMetadata,
		GenerateProofRemotely:  hinkal.GenerateProofRemotely(),
		FeeStructure:           types.ZeroFeeStructure(),
		ChainID:                chainID,
	})
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	if constants.IsTronLike(chainID) {
		if returnTxData {
			return types.TransactionRequest{}, "", errTronReturnTxDataNotImplemented
		}
		client, err := hinkal.GetTronWeb()
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
		txid, err := tron.TransactCallDirectTron(ctx, client, chainID, tron.TransactCallDirectTronParams{
			Amounts:         amountChanges,
			TokensToApprove: erc20Tokens,
			ZkCallData:      proof.ZkCallData,
			CircomData:      proof.CircomData,
			DimData:         proof.DimData,
			PreEstimateGas:  preEstimateGas,
		})
		return types.TransactionRequest{}, txid, err
	}

	adapter, err := hinkal.GetProviderAdapter(&chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	return web3.TransactCallDirect(ctx, adapter, chainID, web3.TransactCallDirectParams{
		Amounts:         amountChanges,
		TokensToApprove: erc20Tokens,
		ZkCallData:      proof.ZkCallData,
		CircomData:      proof.CircomData,
		DimData:         proof.DimData,
		PreEstimateGas:  preEstimateGas,
		ReturnTxData:    returnTxData,
	})
}
