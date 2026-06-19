package transactions

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	"github.com/gioeba/go_sdk_test/internal/functions/balance"
	"github.com/gioeba/go_sdk_test/internal/functions/fees"
	"github.com/gioeba/go_sdk_test/internal/functions/onchainutxos"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	"github.com/gioeba/go_sdk_test/internal/functions/tron"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

type recipientUtxo struct {
	recipientAddress string
	utxo             *utxo.Utxo
}

type depositOnChainProofResult struct {
	proof                                    snarkjs.ConstructZkProofResult
	utxoAmounts                              []*big.Int
	totalAmount                              *big.Int
	hinkalWrapperAddress                     string
	depositOnChainUtxosExternalActionAddress string
}

func depositAndWithdrawUtxoAmounts(recipientAmounts []*big.Int, feeStructure types.FeeStructure) ([]*big.Int, *big.Int) {
	utxoAmounts := make([]*big.Int, len(recipientAmounts))
	totalAmount := new(big.Int)
	for i, amount := range recipientAmounts {
		totalFee := fees.CalculateTotalFee(amount, feeStructure)
		utxoAmounts[i] = new(big.Int).Add(amount, totalFee)
		totalAmount.Add(totalAmount, utxoAmounts[i])
	}
	return utxoAmounts, totalAmount
}

func prepareDepositOnChainUtxosZkProof(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	recipientAmounts []*big.Int,
	feeStructure types.FeeStructure,
) (depositOnChainProofResult, error) {
	tokenAddress := erc20Token.Erc20TokenAddress
	utxoAmounts, totalAmount := depositAndWithdrawUtxoAmounts(recipientAmounts, feeStructure)
	metadata, err := web3.EncodeUint256Array(utxoAmounts)
	if err != nil {
		return depositOnChainProofResult{}, err
	}

	contractData, err := constants.GetContractData(chainID)
	if err != nil {
		return depositOnChainProofResult{}, err
	}
	if contractData.HinkalWrapperAddress == "" || contractData.DepositOnChainUtxosExternalActionAddress == "" {
		return depositOnChainProofResult{}, errors.New("transactions: deposit-on-chain contract data not set")
	}

	erc20Addresses := []string{tokenAddress}
	amountChanges := []*big.Int{big.NewInt(0)}
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, erc20Addresses, amountChanges, 2, nil, true, false)
	if err != nil {
		return depositOnChainProofResult{}, err
	}

	outputUtxosArray := make([][]*utxo.Utxo, len(inputUtxosArray))
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	for i := range inputUtxosArray {
		outputUtxos, err := pretransaction.OutputUtxoProcessing(hinkal.GetUserKeys(), inputUtxosArray[i], amountChanges[i], timeStamp, true, "", nil)
		if err != nil {
			return depositOnChainProofResult{}, err
		}
		outputUtxosArray[i] = outputUtxos
	}

	proof, err := snarkjs.ConstructZkProof(ctx, snarkjs.ConstructZkProofParams{
		MerkleTree:             hinkal.MerkleTree(chainID),
		InputUtxos:             inputUtxosArray,
		OutputUtxos:            outputUtxosArray,
		UserKeys:               hinkal.GetUserKeys(),
		ExternalActionID:       types.ExternalActionDepositOnChainUtxos,
		ExternalAddress:        contractData.DepositOnChainUtxosExternalActionAddress,
		ExternalActionMetadata: []string{metadata},
		GenerateProofRemotely:  hinkal.GenerateProofRemotely(),
		FeeStructure:           types.ZeroFeeStructure(),
		Relay:                  constants.ZeroAddress,
		ChainID:                chainID,
		OnChainCreation:        []bool{true},
		OriginalSender:         contractData.HinkalWrapperAddress,
	})
	if err != nil {
		return depositOnChainProofResult{}, err
	}

	return depositOnChainProofResult{
		proof:                                    proof,
		utxoAmounts:                              utxoAmounts,
		totalAmount:                              totalAmount,
		hinkalWrapperAddress:                     contractData.HinkalWrapperAddress,
		depositOnChainUtxosExternalActionAddress: contractData.DepositOnChainUtxosExternalActionAddress,
	}, nil
}

func matchRecipientUtxos(recipientAddresses []string, utxoAmounts []*big.Int, depositedUtxos []*utxo.Utxo) ([]recipientUtxo, error) {
	available := append([]*utxo.Utxo{}, depositedUtxos...)
	out := make([]recipientUtxo, 0, len(recipientAddresses))
	for i, recipientAddress := range recipientAddresses {
		var matchIndex = -1
		for j, candidate := range available {
			if candidate.Amount.Cmp(utxoAmounts[i]) == 0 {
				matchIndex = j
				break
			}
		}
		if matchIndex < 0 {
			return nil, fmt.Errorf("transactions: could not find newly created UTXO with amount %s for recipient %s", utxoAmounts[i], recipientAddress)
		}
		out = append(out, recipientUtxo{recipientAddress: recipientAddress, utxo: available[matchIndex]})
		available = append(available[:matchIndex], available[matchIndex+1:]...)
	}
	return out, nil
}

func HinkalDepositOnChainUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	feeStructure types.FeeStructure,
	hashedEthereumAddress string,
	preEstimateGas bool,
) ([]recipientUtxo, string, string, error) {
	prepared, err := prepareDepositOnChainUtxosZkProof(ctx, hinkal, chainID, erc20Token, recipientAmounts, feeStructure)
	if err != nil {
		return nil, "", "", err
	}

	statusResp, err := api.UpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseBeforeDeposit,
	})
	if err != nil {
		return nil, "", "", err
	}

	var depositTxHash string
	if constants.IsTronLike(chainID) {
		client, err := hinkal.GetTronWeb()
		if err != nil {
			return nil, "", "", err
		}
		depositTxHash, err = tron.TransactCallDirectTron(ctx, client, chainID, tron.TransactCallDirectTronParams{
			Amounts:          []*big.Int{prepared.totalAmount},
			TokensToApprove:  []types.ERC20Token{erc20Token},
			ZkCallData:       prepared.proof.ZkCallData,
			CircomData:       prepared.proof.CircomData,
			DimData:          prepared.proof.DimData,
			ContractApproval: prepared.depositOnChainUtxosExternalActionAddress,
			ContractTransact: prepared.hinkalWrapperAddress,
			PreEstimateGas:   preEstimateGas,
		})
	} else {
		adapter, err := hinkal.GetProviderAdapter(&chainID)
		if err != nil {
			return nil, "", "", err
		}
		_, depositTxHash, err = web3.TransactCallDirect(ctx, adapter, chainID, web3.TransactCallDirectParams{
			Amounts:          []*big.Int{prepared.totalAmount},
			TokensToApprove:  []types.ERC20Token{erc20Token},
			ZkCallData:       prepared.proof.ZkCallData,
			CircomData:       prepared.proof.CircomData,
			DimData:          prepared.proof.DimData,
			ContractApproval: prepared.depositOnChainUtxosExternalActionAddress,
			ContractTransact: prepared.hinkalWrapperAddress,
			PreEstimateGas:   preEstimateGas,
			ReturnTxData:     false,
		})
	}
	if err != nil {
		return nil, "", "", err
	}
	if depositTxHash == "" {
		return nil, "", "", errors.New("transactions: deposit transaction hash not found")
	}
	if _, err := hinkal.WaitForTransaction(ctx, chainID, depositTxHash, 1); err != nil {
		return nil, "", "", err
	}
	fetchClient, err := hinkal.GetFetchClient(chainID)
	if err != nil {
		return nil, "", "", err
	}
	receipt, err := web3.FetchTransactionReceiptWithRetry(ctx, fetchClient, depositTxHash)
	if err != nil {
		return nil, "", "", err
	}

	_, _ = api.SafeUpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ID:                    statusResp.ID,
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseAfterDeposit,
		DepositTxHash:         depositTxHash,
	})

	depositedUtxos, err := onchainutxos.DecodeFromReceipt(receipt, hinkal.GetUserKeys(), chainID, erc20Token.Erc20TokenAddress)
	if err != nil {
		return nil, "", "", err
	}
	if len(depositedUtxos) == 0 {
		return nil, "", "", errNoDepositedOnChainUtxos
	}
	userDepositedUtxos, err := matchRecipientUtxos(recipientAddresses, prepared.utxoAmounts, depositedUtxos)
	if err != nil {
		return nil, "", "", err
	}
	return userDepositedUtxos, statusResp.ID, depositTxHash, nil
}
