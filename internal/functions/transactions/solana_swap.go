package transactions

import (
	"context"
	"errors"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	solana "github.com/gagliardetto/solana-go"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	"github.com/gioeba/go_sdk_test/internal/functions/balance"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	solanautils "github.com/gioeba/go_sdk_test/internal/functions/solana"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

const solanaSwapDefaultSlippagePercent = 0.7

var (
	errSolanaSwapTwoTokens = errors.New("transactions: Solana swap requires exactly two tokens")
	errSwapLowOutputAmount = errors.New("transactions: swap output amount is below the relayer fee")
)

func getSolanaSwapInputAndOutputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
) ([][]*utxo.Utxo, [][]*utxo.Utxo, error) {
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, 6, nil, false, false)
	if err != nil {
		return nil, nil, err
	}
	userKeys := hinkal.GetUserKeys()
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	outputUtxosArray := make([][]*utxo.Utxo, len(mintAddresses))
	for i := range mintAddresses {
		outputUtxos, err := pretransaction.OutputUtxoProcessing(userKeys, inputUtxosArray[i], amountChanges[i], timeStamp, true, "", nil)
		if err != nil {
			return nil, nil, err
		}
		outputUtxosArray[i] = []*utxo.Utxo{outputUtxos[0]}
	}
	return inputUtxosArray, outputUtxosArray, nil
}

func solanaSwapEncryptedOutputs(outputUtxosArray [][]*utxo.Utxo) ([][]byte, [][]int, error) {
	encryptedOutputs, err := snarkjs.CalcEncryptedOutputs(outputUtxosArray)
	if err != nil {
		return nil, nil, err
	}
	bytesArr := make([][]byte, len(encryptedOutputs))
	intsArr := make([][]int, len(encryptedOutputs))
	for i, tokenOutputs := range encryptedOutputs {
		if len(tokenOutputs) == 0 {
			return nil, nil, errSolanaWithdrawMissingOutput
		}
		row := common.FromHex(tokenOutputs[0])
		bytesArr[i] = row
		ints := make([]int, len(row))
		for j, b := range row {
			ints[j] = int(b)
		}
		intsArr[i] = ints
	}
	return bytesArr, intsArr, nil
}

func HinkalSolanaSwap(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChangesBase []*big.Int,
	swapperAccountSalt *big.Int,
	instructionLists []api.OKXSwapResponseInstruction,
	addressLookupTableAccount []string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	if len(erc20Tokens) != 2 || len(amountChangesBase) != 2 {
		return "", errSolanaSwapTwoTokens
	}

	recipient, err := solana.PublicKeyFromBase58(constants.SolanaNativeAddress)
	if err != nil {
		return "", err
	}
	hinkalAddressStr, err := constants.HinkalAddress(chainID)
	if err != nil {
		return "", err
	}
	hinkalProgramAddress, err := solana.PublicKeyFromBase58(hinkalAddressStr)
	if err != nil {
		return "", err
	}
	originalDeployerStr, err := constants.OriginalDeployer(chainID)
	if err != nil {
		return "", err
	}
	originalDeployer, err := solana.PublicKeyFromBase58(originalDeployerStr)
	if err != nil {
		return "", err
	}

	amountChanges := copyBigInts(amountChangesBase)
	mintAddresses := tokenAddresses(erc20Tokens)
	if feeToken == "" {
		feeToken = mintAddresses[1]
	}

	var feeStructure types.FeeStructure
	if feeStructureOverride != nil {
		feeStructure = *feeStructureOverride
	} else {
		solanaParams := &api.SolanaGasEstimateParams{
			MintTo:         feeToken,
			MintFrom:       mintAddresses[0],
			NullifierCount: pretransaction.CalculateSolanaNullifierCount(ctx, hinkal, chainID, mintAddresses, amountChanges),
		}
		feeStructure, err = pretransaction.GetFeeStructure(ctx, chainID, feeToken, mintAddresses, types.ExternalActionOkx, nil, big.NewInt(constants.HinkalSwapVariableRate), solanaParams)
		if err != nil {
			return "", err
		}
	}

	initialReceiveAmount := amountChanges[1]
	outputToken := erc20Tokens[1]
	outputAmount, err := strconv.ParseFloat(web3.GetAmountInToken(outputToken, initialReceiveAmount), 64)
	if err != nil {
		return "", err
	}
	slippageAmount := outputAmount * solanaSwapDefaultSlippagePercent / 100
	slippageWei, err := web3.GetAmountInWei(outputToken, strconv.FormatFloat(slippageAmount, 'f', outputToken.Decimals, 64))
	if err != nil {
		return "", err
	}
	amountChanges[1] = new(big.Int).Sub(amountChanges[1], slippageWei)

	variableFee := new(big.Int).Quo(new(big.Int).Mul(initialReceiveAmount, feeStructure.VariableRate), big.NewInt(10000))
	totalFee := new(big.Int).Add(variableFee, feeStructure.FlatFee)
	amountChanges[1] = new(big.Int).Sub(amountChanges[1], totalFee)
	if amountChanges[1].Sign() < 0 {
		return "", errSwapLowOutputAmount
	}

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	inputUtxosArray, outputUtxosArray, err := getSolanaSwapInputAndOutputUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges)
	if err != nil {
		return "", err
	}

	shieldedPrivateKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}
	randSeed, err := utils.RandomBigInt(31)
	if err != nil {
		return "", err
	}
	extraRandomization, err := cryptokeys.FindCorrectRandomization(randSeed, shieldedPrivateKey)
	if err != nil {
		return "", err
	}

	encryptedOutputBytes, encryptedOutputInts, err := solanaSwapEncryptedOutputs(outputUtxosArray)
	if err != nil {
		return "", err
	}

	swapperAccount, err := pretransaction.GetSwapperAccountPublicKeyFromSalt(hinkalProgramAddress, originalDeployer, swapperAccountSalt)
	if err != nil {
		return "", err
	}
	hinkalInstructions, remainingAccounts, err := pretransaction.ConvertOKXToHinkalInstructions(instructionLists, swapperAccount)
	if err != nil {
		return "", err
	}

	if err := snarkjs.EnsureAmountChanges(inputUtxosArray, outputUtxosArray, amountChanges); err != nil {
		return "", err
	}

	dimensions := types.DimDataType{
		TokenNumber:     2,
		NullifierAmount: len(inputUtxosArray[0]),
		OutputAmount:    1,
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
		RecipientAddress:      recipient.String(),
		SignerAddress:         relay,
		Dimensions:            dimensions,
		EncryptedOutputs:      encryptedOutputBytes,
		ChainID:               chainID,
		Instructions:          hinkalInstructions,
		RemainingAccounts:     remainingAccounts,
		SwapperAccountSalt:    swapperAccountSalt,
	})
	if err != nil {
		return "", err
	}

	storageAccount, err := pretransaction.GetStorageAccountPublicKey(hinkalProgramAddress, originalDeployer)
	if err != nil {
		return "", err
	}
	storageVault, err := pretransaction.GetStorageVaultPublicKey(hinkalProgramAddress, originalDeployer)
	if err != nil {
		return "", err
	}

	accounts := api.SolanaSwapAccounts{
		Recipient:                 recipient.String(),
		StorageAccount:            storageAccount.String(),
		StorageVault:              storageVault.String(),
		SwapperAccount:            swapperAccount.String(),
		MintFrom:                  nonNativeMint(mintAddresses[0]),
		MintTo:                    nonNativeMint(mintAddresses[1]),
		RemainingAccounts:         remainingAccountsToOKX(remainingAccounts),
		AddressLookupTableAccount: addressLookupTableAccount,
	}

	variableRate := feeStructure.VariableRate.String()
	args := api.SolanaSwapArgs{
		SolanaArgs: api.SolanaArgs{
			ProofAArr:        proof.ProofAArr,
			ProofBArr:        proof.ProofBArr,
			ProofCArr:        proof.ProofCArr,
			PublicInputsArr:  proof.PublicInputsArr,
			EncryptedOutputs: encryptedOutputInts,
			RelayerFee:       feeStructure.FlatFee.String(),
			VariableRate:     &variableRate,
			Dimensions:       dimensions,
		},
		HinkalInstructions: hinkalInstructionsToAPI(hinkalInstructions),
	}

	return web3.SolanaTransactCallRelayer(ctx, api.SolanaTransactionBody{
		ChainID:                  chainID,
		RelayAddress:             relay,
		FunctionName:             "swap",
		Args:                     args,
		Accounts:                 accounts,
		CommitmentValidationData: proof.CommitmentValidationData,
	})
}

func nonNativeMint(mintAddress string) *string {
	if mintAddress == constants.SolanaNativeAddress {
		return nil
	}
	mint := mintAddress
	return &mint
}

func remainingAccountsToOKX(remainingAccounts []solana.AccountMeta) []api.OKXAccount {
	out := make([]api.OKXAccount, len(remainingAccounts))
	for i, acc := range remainingAccounts {
		out[i] = api.OKXAccount{
			IsSigner:   acc.IsSigner,
			IsWritable: acc.IsWritable,
			Pubkey:     acc.PublicKey.String(),
		}
	}
	return out
}

func hinkalInstructionsToAPI(instructions []solanautils.HinkalInstruction) []api.SolanaHinkalInstruction {
	out := make([]api.SolanaHinkalInstruction, len(instructions))
	for i, inst := range instructions {
		accountIndexes := make([]int, len(inst.AccountIndexes))
		for j, b := range inst.AccountIndexes {
			accountIndexes[j] = int(b)
		}
		data := make([]int, len(inst.Data))
		for j, b := range inst.Data {
			data[j] = int(b)
		}
		out[i] = api.SolanaHinkalInstruction{
			AccountIndexes: accountIndexes,
			Data:           data,
			ProgramIndex:   inst.ProgramIndex,
		}
	}
	return out
}
