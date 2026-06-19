package transactions

import (
	"context"
	"math/big"
	"strings"

	solana "github.com/gagliardetto/solana-go"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	"github.com/gioeba/go_sdk_test/internal/functions/fees"
	"github.com/gioeba/go_sdk_test/internal/functions/onchainutxos"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	solanautils "github.com/gioeba/go_sdk_test/internal/functions/solana"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

func resolveSolanaDepositAndWithdrawFeeStructure(
	ctx context.Context,
	chainID int,
	mintAddress string,
	feeStructureOverride *types.FeeStructure,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (types.FeeStructure, error) {
	if feeStructureOverride != nil {
		return privateSendFeeStructure(*feeStructureOverride), nil
	}
	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		mintAddress,
		[]string{mintAddress},
		types.ExternalActionTransact,
		nil,
		big.NewInt(constants.HinkalPrivateSendVariableRate),
		solanaTransactionParams,
	)
	if err != nil {
		return types.FeeStructure{}, err
	}
	return privateSendFeeStructure(feeStructure), nil
}

func hinkalSolanaMultiPaymentDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	token types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	feeStructure types.FeeStructure,
	hashedEthereumAddress string,
) ([]recipientUtxo, string, string, error) {
	amounts, _ := depositAndWithdrawUtxoAmounts(recipientAmounts, feeStructure)
	structures, err := getProoflessStealthAddressStructures(hinkal, len(amounts), nil)
	if err != nil {
		return nil, "", "", err
	}
	if err := validateSolanaDepositArgs(amounts, structures); err != nil {
		return nil, "", "", err
	}

	programID, err := solana.PublicKeyFromBase58(hinkal.HinkalAddress(chainID))
	if err != nil {
		return nil, "", "", err
	}
	originalDeployerStr, err := constants.OriginalDeployer(chainID)
	if err != nil {
		return nil, "", "", err
	}
	originalDeployer, err := solana.PublicKeyFromBase58(originalDeployerStr)
	if err != nil {
		return nil, "", "", err
	}
	signer, err := hinkal.GetSolanaPublicKey(ctx)
	if err != nil {
		return nil, "", "", err
	}
	connection, err := hinkal.GetSolanaConnection()
	if err != nil {
		return nil, "", "", err
	}

	instruction, err := buildMultiPaymentDepositInstruction(programID, signer, originalDeployer, token.Erc20TokenAddress, amounts, structures, true)
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

	signature, err := signAndSendSolanaInstructions(ctx, hinkal, programID, connection, signer, []solana.Instruction{instruction})
	if err != nil {
		return nil, "", "", err
	}
	if _, err := hinkal.WaitForTransaction(ctx, chainID, signature, 1); err != nil {
		return nil, "", "", err
	}
	tx, err := solanautils.FetchTransactionWithRetry(ctx, chainID, signature)
	if err != nil {
		return nil, "", "", err
	}

	_, _ = api.SafeUpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ID:                    statusResp.ID,
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseAfterDeposit,
		DepositTxHash:         signature,
	})

	formattedMint, err := solanautils.FormatMintAddress(token.Erc20TokenAddress)
	if err != nil {
		return nil, "", "", err
	}
	depositedUtxos, err := onchainutxos.DecodeSolanaFromTransaction(tx, hinkal.GetUserKeys(), formattedMint.CompressedAddress)
	if err != nil {
		return nil, "", "", err
	}
	if len(depositedUtxos) == 0 {
		return nil, "", "", errNoDepositedOnChainUtxos
	}
	userDepositedUtxos, err := matchRecipientUtxos(recipientAddresses, amounts, depositedUtxos)
	if err != nil {
		return nil, "", "", err
	}
	return userDepositedUtxos, statusResp.ID, signature, nil
}

func buildSolanaDepositAndWithdrawZeroUtxo(
	hinkal ihinkal.HinkalInternal,
	mintAddress string,
	source *utxo.Utxo,
	timeStamp string,
) (*utxo.Utxo, error) {
	nativeMint, err := solanautils.FormatMintAddress(constants.SolanaNativeAddress)
	if err != nil {
		return nil, err
	}
	shieldedPrivateKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	params := types.UtxoParams{
		Amount:            big.NewInt(0),
		MintAddress:       mintAddress,
		Erc20TokenAddress: nativeMint.CompressedAddress,
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

func hinkalSolanaWithdrawBatch(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	token types.ERC20Token,
	userDepositedUtxos []recipientUtxo,
	feeStructure types.FeeStructure,
	hashedEthereumAddress string,
	recipientAmounts []*big.Int,
	statusID string,
	txCompletionTime *int,
) (string, error) {
	if len(userDepositedUtxos) == 0 {
		return "", errNoDepositedOnChainUtxos
	}
	mintAddress := token.Erc20TokenAddress
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	withdrawTimeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	transactions := make([]api.SolanaTransactionBody, 0, len(userDepositedUtxos))

	shieldedPrivateKey, err := hinkal.GetUserKeys().GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}
	for i, item := range userDepositedUtxos {
		zeroUtxo, err := buildSolanaDepositAndWithdrawZeroUtxo(hinkal, mintAddress, item.utxo, withdrawTimeStamp)
		if err != nil {
			return "", err
		}
		withdrawInputUtxos := []*utxo.Utxo{item.utxo, zeroUtxo}
		withdrawOutputUtxos := []*utxo.Utxo{zeroUtxo}
		randSeed, err := utils.RandomBigInt(31)
		if err != nil {
			return "", err
		}
		extraRandomization, err := cryptokeys.FindCorrectRandomization(randSeed, shieldedPrivateKey)
		if err != nil {
			return "", err
		}
		encryptedOutputBytes, encryptedOutputInts, err := solanaEncryptedOutputBytes(withdrawOutputUtxos)
		if err != nil {
			return "", err
		}
		inputUtxosArray := [][]*utxo.Utxo{withdrawInputUtxos}
		outputUtxosArray := [][]*utxo.Utxo{withdrawOutputUtxos}
		dimensions := types.DimDataType{
			TokenNumber:     1,
			NullifierAmount: len(withdrawInputUtxos),
			OutputAmount:    len(withdrawOutputUtxos),
		}
		finalFeeStructure := fees.CalculateModifiedFeeStructure(ctx, chainID, token, recipientAmounts[i], feeStructure)
		proof, err := snarkjs.ConstructSolanaZkProof(ctx, snarkjs.ConstructSolanaZkProofParams{
			GenerateProofRemotely: hinkal.GenerateProofRemotely(),
			MerkleTree:            hinkal.MerkleTree(chainID),
			UserKeys:              hinkal.GetUserKeys(),
			MintAddresses:         []string{mintAddress},
			InputUtxos:            inputUtxosArray,
			OutputUtxos:           outputUtxosArray,
			ExtraRandomization:    extraRandomization,
			RelayerFee:            finalFeeStructure.FlatFee,
			VariableRate:          finalFeeStructure.VariableRate,
			RecipientAddress:      item.recipientAddress,
			SignerAddress:         relay,
			Dimensions:            dimensions,
			EncryptedOutputs:      encryptedOutputBytes,
			ChainID:               chainID,
		})
		if err != nil {
			return "", err
		}

		accounts := api.SolanaTransactAccounts{Recipient: item.recipientAddress}
		if !strings.EqualFold(mintAddress, constants.SolanaNativeAddress) {
			accounts.Mint = mintAddress
		}
		transactions = append(transactions, api.SolanaTransactionBody{
			ChainID:         chainID,
			RelayAddress:    relay,
			FunctionName:    "transact",
			RecipientAmount: recipientAmounts[i].String(),
			Args: api.SolanaArgs{
				ProofAArr:        proof.ProofAArr,
				ProofBArr:        proof.ProofBArr,
				ProofCArr:        proof.ProofCArr,
				PublicInputsArr:  proof.PublicInputsArr,
				EncryptedOutputs: encryptedOutputInts,
				RelayerFee:       finalFeeStructure.FlatFee.String(),
				Dimensions:       dimensions,
			},
			Accounts:                 accounts,
			CommitmentValidationData: proof.CommitmentValidationData,
		})
	}

	_, _ = api.SafeUpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ID:                    statusID,
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseBeforeScheduleWithdraw,
	})
	scheduleID, err := web3.SolanaTransactCallRelayerBatch(ctx, chainID, transactions, hashedEthereumAddress, txCompletionTime, "", "")
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

func HinkalSolanaDepositAndWithdraw(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
) (types.DepositAndSendExtendedResult, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if !constants.IsSolanaLike(chainID) {
		return types.DepositAndSendExtendedResult{}, errNotSolanaChain
	}
	if err := validateDepositAndWithdrawArgs(erc20Tokens, recipientAmounts, recipientAddresses); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	token := erc20Tokens[0]
	rawEthereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	hashedEthereumAddress := utils.HashEthereumAddress(rawEthereumAddress)
	solanaParams := &api.SolanaGasEstimateParams{
		MintTo:         token.Erc20TokenAddress,
		NullifierCount: 1,
	}
	feeStructure, err := resolveSolanaDepositAndWithdrawFeeStructure(ctx, chainID, token.Erc20TokenAddress, feeStructureOverride, solanaParams)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	userDepositedUtxos, statusID, depositTxHash, err := hinkalSolanaMultiPaymentDeposit(
		ctx,
		hinkal,
		chainID,
		token,
		recipientAmounts,
		recipientAddresses,
		feeStructure,
		hashedEthereumAddress,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if err := waitForDepositedUtxosInMerkleTree(ctx, hinkal, chainID, userDepositedUtxos); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	scheduleID, err := hinkalSolanaWithdrawBatch(
		ctx,
		hinkal,
		chainID,
		token,
		userDepositedUtxos,
		feeStructure,
		hashedEthereumAddress,
		recipientAmounts,
		statusID,
		txCompletionTime,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	return types.DepositAndSendExtendedResult{DepositTxHash: depositTxHash, ScheduleID: scheduleID}, nil
}
