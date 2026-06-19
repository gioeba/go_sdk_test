package transactions

import (
	"context"
	"errors"
	"math/big"
	"strconv"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	"github.com/gioeba/go_sdk_test/internal/functions/balance"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	"github.com/gioeba/go_sdk_test/internal/functions/tron"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

var (
	errDepositNotImplemented          = errors.New("transactions: deposit is not implemented for Solana chains")
	errTronReturnTxDataNotImplemented = errors.New("transactions: Tron returnTxData is not implemented")
)

type transactionParameters struct {
	externalActionID       types.ExternalActionID
	externalAddress        string
	externalActionMetadata []string
}

func getTransactionParameters(ctx context.Context, hinkal ihinkal.HinkalInternal, chainID int) (transactionParameters, error) {
	ethereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return transactionParameters{}, err
	}
	if constants.IsTronLike(chainID) {
		ethereumAddress, err = utils.AddressToHexFormat(ethereumAddress)
		if err != nil {
			return transactionParameters{}, err
		}
	}
	return transactionParameters{
		externalActionID:       types.ExternalActionZero,
		externalAddress:        ethereumAddress,
		externalActionMetadata: nil,
	}, nil
}

func getInputAndOutputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
) (inputUtxosArray, outputUtxosArray [][]*utxo.Utxo, err error) {
	inputUtxosArray, err = balance.AddPaddingToUtxos(ctx, hinkal, chainID, erc20Addresses, amountChanges, 0, nil, false, false)
	if err != nil {
		return nil, nil, err
	}

	userKeys := hinkal.GetUserKeys()
	timeStamp := strconv.FormatInt(utils.GetCurrentTimeInSeconds(), 10)
	outputUtxosArray = make([][]*utxo.Utxo, 0, len(erc20Addresses))
	for i := range erc20Addresses {
		outputUtxos, err := pretransaction.OutputUtxoProcessing(userKeys, inputUtxosArray[i], amountChanges[i], timeStamp, true, "", nil)
		if err != nil {
			return nil, nil, err
		}
		outputUtxosArray = append(outputUtxosArray, outputUtxos)
	}
	return inputUtxosArray, outputUtxosArray, nil
}

func HinkalDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChanges []*big.Int,
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

	erc20Addresses := make([]string, len(erc20Tokens))
	for i, token := range erc20Tokens {
		erc20Addresses[i] = token.Erc20TokenAddress
	}

	params, err := getTransactionParameters(ctx, hinkal, chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	inputUtxosArray, outputUtxosArray, err := getInputAndOutputUtxos(ctx, hinkal, chainID, erc20Addresses, amountChanges)
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
