package transactions

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/contractabi"
	"github.com/gioeba/go_sdk_test/cryptokeys"
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
	errWithdrawStuckNoToken       = errors.New("transactions: withdrawStuckUtxos action: no token found")
	errWithdrawStuckTooManyTokens = errors.New("transactions: withdrawStuckUtxos supports one token")
	errWithdrawStuckNoUtxos       = errors.New("transactions: withdrawStuckUtxos no stuck UTXOs found")
)

func countStuckUtxoAmount(utxos []*utxo.Utxo) *big.Int {
	total := new(big.Int)
	for _, u := range utxos {
		if u.Amount != nil {
			total.Add(total, u.Amount)
		}
	}
	return total
}

func topPositiveUtxosForToken(
	inputUtxos []*utxo.Utxo,
	chainID int,
	tokenAddress string,
) ([]*utxo.Utxo, error) {
	filtered, err := positiveUtxosForToken(inputUtxos, chainID, tokenAddress)
	if err != nil {
		return nil, err
	}
	return sortAndCapStuckUtxos(filtered), nil
}

func positiveUtxosForToken(
	inputUtxos []*utxo.Utxo,
	chainID int,
	tokenAddress string,
) ([]*utxo.Utxo, error) {
	filtered := make([]*utxo.Utxo, 0, len(inputUtxos))
	for _, u := range inputUtxos {
		if u.Amount == nil || u.Amount.Sign() <= 0 {
			continue
		}
		candidateTokenAddress, err := u.GetTokenAddress(chainID)
		if err != nil {
			continue
		}
		if strings.EqualFold(candidateTokenAddress, tokenAddress) {
			filtered = append(filtered, u)
		}
	}
	return filtered, nil
}

func sortAndCapStuckUtxos(filtered []*utxo.Utxo) []*utxo.Utxo {
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Amount.Cmp(filtered[j].Amount) > 0
	})
	if len(filtered) > 6 {
		filtered = filtered[:6]
	}
	return filtered
}

func filterOnChainUnspentUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	inputUtxos []*utxo.Utxo,
) ([]*utxo.Utxo, error) {
	if constants.IsSolanaLike(chainID) || len(inputUtxos) == 0 {
		return inputUtxos, nil
	}

	parsedABI, err := contractabi.Hinkal(chainID)
	if err != nil {
		return nil, err
	}
	client, err := hinkal.GetFetchClient(chainID)
	if err != nil {
		return nil, err
	}
	hinkalAddress := hinkal.HinkalAddress(chainID)
	if hinkalAddress == "" {
		hinkalAddress, err = constants.HinkalAddress(chainID)
		if err != nil {
			return nil, err
		}
	}
	contractAddress := common.HexToAddress(hinkalAddress)
	knownNullifiers := hinkal.Nullifiers(chainID)

	filtered := make([]*utxo.Utxo, 0, len(inputUtxos))
	for _, inputUtxo := range inputUtxos {
		nullifier, err := inputUtxo.GetNullifier()
		if err != nil {
			return nil, err
		}
		nullifierBig, err := utils.ParseBigInt(nullifier)
		if err != nil {
			return nil, err
		}
		data, err := parsedABI.Pack("nullifiers", nullifierBig)
		if err != nil {
			return nil, err
		}
		out, err := client.CallContract(ctx, ethereum.CallMsg{To: &contractAddress, Data: data}, nil)
		if err != nil {
			return nil, err
		}
		values, err := parsedABI.Unpack("nullifiers", out)
		if err != nil {
			return nil, err
		}
		if len(values) == 0 {
			return nil, fmt.Errorf("transactions: empty nullifiers result")
		}
		spent, ok := values[0].(bool)
		if !ok {
			return nil, fmt.Errorf("transactions: unexpected nullifiers result type %T", values[0])
		}
		if spent {
			if knownNullifiers != nil {
				knownNullifiers[utils.ToBeHex(nullifierBig)] = struct{}{}
			}
			continue
		}
		filtered = append(filtered, inputUtxo)
	}
	return filtered, nil
}

func positiveStuckUtxosForToken(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	tokenAddress string,
	ethAddress string,
) ([]*utxo.Utxo, error) {
	inputUtxos, err := balance.GetInputUtxoAndBalanceOfStuckUtxos(ctx, balance.InputUtxoParams{
		Hinkal:                hinkal,
		ChainID:               chainID,
		EthAddress:            ethAddress,
		AllowRemoteDecryption: hinkal.GenerateProofRemotely(),
	})
	if err != nil {
		return nil, err
	}
	positiveUtxos, err := positiveUtxosForToken(inputUtxos, chainID, tokenAddress)
	if err != nil {
		return nil, err
	}
	unspentUtxos, err := filterOnChainUnspentUtxos(ctx, hinkal, chainID, positiveUtxos)
	if err != nil {
		return nil, err
	}
	return sortAndCapStuckUtxos(unspentUtxos), nil
}

func getStuckInputAndOutputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	tokenAddresses []string,
	amountChanges []*big.Int,
) ([][]*utxo.Utxo, [][]*utxo.Utxo, error) {
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, tokenAddresses, amountChanges, 0, nil, false, true)
	if err != nil {
		return nil, nil, err
	}

	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	outputUtxosArray := make([][]*utxo.Utxo, 0, len(tokenAddresses))
	for i := range tokenAddresses {
		outputUtxos, err := pretransaction.OutputUtxoProcessing(hinkal.GetUserKeys(), inputUtxosArray[i], amountChanges[i], timeStamp, true, "", nil)
		if err != nil {
			return nil, nil, err
		}
		outputUtxosArray = append(outputUtxosArray, outputUtxos)
	}
	return inputUtxosArray, outputUtxosArray, nil
}

func getSolanaStuckInputAndOutputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
) ([]*utxo.Utxo, []*utxo.Utxo, error) {
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, 6, nil, false, true)
	if err != nil {
		return nil, nil, err
	}
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	outputUtxos, err := pretransaction.OutputUtxoProcessing(hinkal.GetUserKeys(), inputUtxosArray[0], amountChanges[0], timeStamp, true, "", nil)
	if err != nil {
		return nil, nil, err
	}
	return inputUtxosArray[0], outputUtxos, nil
}

func withdrawSingleStuckToken(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	totalAmountInUtxos *big.Int,
	recipientAddressRaw string,
) (string, *big.Int, error) {
	tokenAddress := erc20Token.Erc20TokenAddress
	tokenAddresses := []string{tokenAddress}

	feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, tokenAddress, tokenAddresses, types.ExternalActionTransact, nil, nil, nil)
	if err != nil {
		return "", nil, err
	}
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", nil, err
	}

	recipientAddress, err := utils.AddressToHexFormat(recipientAddressRaw)
	if err != nil {
		return "", nil, err
	}
	amountToRecipient := new(big.Int).Sub(totalAmountInUtxos, feeStructure.FlatFee)
	if amountToRecipient.Sign() <= 0 {
		return "", nil, fmt.Errorf("insufficient balance to cover fee. Balance: %s, Fee: %s", totalAmountInUtxos, feeStructure.FlatFee)
	}

	amountChanges := []*big.Int{new(big.Int).Neg(amountToRecipient)}
	if err := pretransaction.MergeWithFeeStructure(chainID, &tokenAddresses, &amountChanges, feeStructure); err != nil {
		return "", nil, err
	}

	inputUtxosArray, outputUtxosArray, err := getStuckInputAndOutputUtxos(ctx, hinkal, chainID, tokenAddresses, amountChanges)
	if err != nil {
		return "", nil, err
	}

	proof, err := snarkjs.ConstructZkProof(ctx, snarkjs.ConstructZkProofParams{
		MerkleTree:             hinkal.MerkleTree(chainID),
		InputUtxos:             inputUtxosArray,
		OutputUtxos:            outputUtxosArray,
		UserKeys:               hinkal.GetUserKeys(),
		ExternalActionID:       types.ExternalActionZero,
		ExternalAddress:        recipientAddress,
		ExternalActionMetadata: nil,
		GenerateProofRemotely:  hinkal.GenerateProofRemotely(),
		FeeStructure:           feeStructure,
		Relay:                  relay,
		ChainID:                chainID,
	})
	if err != nil {
		return "", nil, err
	}

	var tronProofSignature *api.TronProofSignature
	if constants.IsTronLike(chainID) {
		signature, err := tron.ReorderZkCallData(ctx, &proof.ZkCallData, proof.DimData, proof.CircomData, true)
		if err != nil {
			return "", nil, err
		}
		tronProofSignature = &signature
	}

	txHash, err := web3.TransactCallRelayer(ctx, chainID, proof.ZkCallData, proof.DimData, proof.CircomData, proof.CommitmentValidationData, false, tronProofSignature)
	if err != nil {
		return "", nil, err
	}
	return txHash, amountToRecipient, nil
}

func withdrawSingleStuckTokenSolana(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	totalAmountInUtxos *big.Int,
	recipientAddress string,
	nullifierCount int,
) (string, *big.Int, error) {
	mintAddresses := []string{erc20Token.Erc20TokenAddress}
	feeStructure, err := pretransaction.GetFeeStructure(ctx, chainID, mintAddresses[0], mintAddresses, types.ExternalActionTransact, nil, nil, &api.SolanaGasEstimateParams{
		MintTo:         mintAddresses[0],
		Recipient:      recipientAddress,
		NullifierCount: nullifierCount,
	})
	if err != nil {
		return "", nil, err
	}
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", nil, err
	}

	amountToRecipient := new(big.Int).Sub(totalAmountInUtxos, feeStructure.FlatFee)
	if amountToRecipient.Sign() <= 0 {
		return "", nil, fmt.Errorf("insufficient balance to cover fee. Balance: %s, Fee: %s", totalAmountInUtxos, feeStructure.FlatFee)
	}

	amountChanges := []*big.Int{new(big.Int).Neg(amountToRecipient)}
	amountChanges[0] = new(big.Int).Sub(amountChanges[0], feeStructure.FlatFee)

	inputUtxos, outputUtxos, err := getSolanaStuckInputAndOutputUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges)
	if err != nil {
		return "", nil, err
	}

	shieldedPrivateKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return "", nil, err
	}
	randSeed, err := utils.RandomBigInt(31)
	if err != nil {
		return "", nil, err
	}
	extraRandomization, err := cryptokeys.FindCorrectRandomization(randSeed, shieldedPrivateKey)
	if err != nil {
		return "", nil, err
	}
	encryptedOutputBytes, encryptedOutputInts, err := solanaEncryptedOutputBytes(outputUtxos)
	if err != nil {
		return "", nil, err
	}
	inputUtxosArray := [][]*utxo.Utxo{inputUtxos}
	outputUtxosArray := [][]*utxo.Utxo{outputUtxos}
	if err := snarkjs.EnsureAmountChanges(inputUtxosArray, outputUtxosArray, amountChanges); err != nil {
		return "", nil, err
	}

	dimensions := types.DimDataType{
		TokenNumber:     len(mintAddresses),
		NullifierAmount: len(inputUtxos),
		OutputAmount:    len(outputUtxos),
	}
	proof, err := snarkjs.ConstructSolanaZkProof(ctx, snarkjs.ConstructSolanaZkProofParams{
		GenerateProofRemotely: hinkal.GenerateProofRemotely(),
		MerkleTree:            hinkal.MerkleTree(chainID),
		UserKeys:              hinkal.GetUserKeys(),
		MintAddresses:         mintAddresses,
		InputUtxos:            inputUtxosArray,
		OutputUtxos:           outputUtxosArray,
		ExtraRandomization:    extraRandomization,
		RelayerFee:            feeStructure.FlatFee,
		VariableRate:          feeStructure.VariableRate,
		RecipientAddress:      recipientAddress,
		SignerAddress:         relay,
		Dimensions:            dimensions,
		EncryptedOutputs:      encryptedOutputBytes,
		ChainID:               chainID,
	})
	if err != nil {
		return "", nil, err
	}

	accounts := api.SolanaTransactAccounts{Recipient: recipientAddress}
	if !strings.EqualFold(mintAddresses[0], constants.SolanaNativeAddress) {
		accounts.Mint = mintAddresses[0]
	}

	txHash, err := web3.SolanaTransactCallRelayer(ctx, api.SolanaTransactionBody{
		ChainID:      chainID,
		RelayAddress: relay,
		FunctionName: "transact",
		Args: api.SolanaArgs{
			ProofAArr:        proof.ProofAArr,
			ProofBArr:        proof.ProofBArr,
			ProofCArr:        proof.ProofCArr,
			PublicInputsArr:  proof.PublicInputsArr,
			EncryptedOutputs: encryptedOutputInts,
			RelayerFee:       feeStructure.FlatFee.String(),
			Dimensions:       dimensions,
		},
		Accounts:                 accounts,
		CommitmentValidationData: proof.CommitmentValidationData,
	})
	if err != nil {
		return "", nil, err
	}
	return txHash, amountToRecipient, nil
}

func withdrawStuckTokenByChainID(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	totalAmount *big.Int,
	recipientAddress string,
	nullifierCount int,
) (string, *big.Int, error) {
	if totalAmount.Sign() == 0 {
		return "", nil, errWithdrawStuckNoUtxos
	}
	if constants.IsSolanaLike(chainID) {
		return withdrawSingleStuckTokenSolana(ctx, hinkal, chainID, erc20Token, totalAmount, recipientAddress, nullifierCount)
	}
	return withdrawSingleStuckToken(ctx, hinkal, chainID, erc20Token, totalAmount, recipientAddress)
}

func HinkalWithdrawStuckUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	recipientAddress string,
) ([]string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return nil, err
	}
	if len(erc20Tokens) == 0 {
		return nil, errWithdrawStuckNoToken
	}
	if len(erc20Tokens) > 1 {
		return nil, errWithdrawStuckTooManyTokens
	}

	ethAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return nil, err
	}
	erc20Token := erc20Tokens[0]
	results := []string{}

	for {
		if err := hinkal.ResetMerkleTreesIfNecessary(ctx, chainID); err != nil {
			return results, err
		}

		stuckUtxos, err := positiveStuckUtxosForToken(ctx, hinkal, chainID, erc20Token.Erc20TokenAddress, ethAddress)
		if err != nil {
			return results, err
		}
		totalAmount := countStuckUtxoAmount(stuckUtxos)
		if totalAmount.Sign() == 0 {
			break
		}

		txHash, _, err := withdrawStuckTokenByChainID(ctx, hinkal, chainID, erc20Token, totalAmount, recipientAddress, len(stuckUtxos))
		if err != nil {
			return results, err
		}
		if _, err := hinkal.WaitForTransaction(ctx, chainID, txHash, 1); err != nil {
			return results, err
		}
		results = append(results, txHash)
	}

	return results, nil
}
