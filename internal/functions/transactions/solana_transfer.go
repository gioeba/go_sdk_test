package transactions

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	errorhandling "github.com/gioeba/go_sdk_test/error-handling"
	"github.com/gioeba/go_sdk_test/internal/functions/balance"
	"github.com/gioeba/go_sdk_test/internal/functions/fees"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

var (
	errSolanaTransferOneMint       = errors.New("transactions: Solana transfer supports one mint per transaction")
	errSolanaTransferMissingOutput = errors.New("transactions: Solana transfer missing output UTXO")
)

func getSolanaTransferInputAndOutputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
	recipientAddress string,
	recipientAmount *big.Int,
) ([]*utxo.Utxo, []*utxo.Utxo, error) {
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, 6, nil, false, false)
	if err != nil {
		return nil, nil, err
	}
	userKeys := hinkal.GetUserKeys()
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	outputUtxos, err := pretransaction.OutputUtxoProcessing(userKeys, inputUtxosArray[0], amountChanges[0], timeStamp, true, recipientAddress, recipientAmount)
	if err != nil {
		return nil, nil, err
	}
	if len(outputUtxos) < 2 {
		return nil, nil, errSolanaTransferMissingOutput
	}
	return inputUtxosArray[0], outputUtxos, nil
}

func solanaTransferEncryptedOutputs(outputUtxos []*utxo.Utxo) ([][]byte, [][]int, error) {
	encryptedOutputs, err := snarkjs.CalcEncryptedOutputs([][]*utxo.Utxo{outputUtxos})
	if err != nil {
		return nil, nil, err
	}
	if len(encryptedOutputs) == 0 || len(encryptedOutputs[0]) != len(outputUtxos) {
		return nil, nil, errSolanaTransferMissingOutput
	}
	bytesArr := make([][]byte, len(encryptedOutputs[0]))
	intsArr := make([][]int, len(encryptedOutputs[0]))
	for i, encryptedOutput := range encryptedOutputs[0] {
		row := common.FromHex(encryptedOutput)
		bytesArr[i] = row
		ints := make([]int, len(row))
		for j, b := range row {
			ints[j] = int(b)
		}
		intsArr[i] = ints
	}
	return bytesArr, intsArr, nil
}

func resolveSolanaTransferFeeStructure(
	ctx context.Context,
	chainID int,
	feeToken string,
	mintAddresses []string,
	feeStructureOverride *types.FeeStructure,
	solanaTransactionParams *api.SolanaGasEstimateParams,
) (types.FeeStructure, error) {
	if feeStructureOverride != nil {
		return privateSendFeeStructure(*feeStructureOverride), nil
	}
	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		feeToken,
		mintAddresses,
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

func HinkalSolanaTransfer(
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
	if !constants.IsSolanaLike(chainID) {
		return "", errNotSolanaChain
	}
	if len(erc20Tokens) != len(amountChangesBase) {
		return "", errTokenAmountLengthMismatch
	}
	if len(erc20Tokens) == 0 {
		return "", errTransferNoToken
	}
	if len(erc20Tokens) > 1 {
		return "", errSolanaTransferOneMint
	}
	if !pretransaction.IsValidPrivateAddress(recipientAddress) {
		return "", errorhandling.ErrRecipientFormatIncorrect
	}

	amountChanges := copyBigInts(amountChangesBase)
	mintAddresses := tokenAddresses(erc20Tokens)
	if feeToken == "" {
		feeToken = mintAddresses[0]
	}
	solanaParams := &api.SolanaGasEstimateParams{
		MintTo:         feeToken,
		NullifierCount: pretransaction.CalculateSolanaNullifierCount(ctx, hinkal, chainID, mintAddresses, amountChanges),
	}
	feeStructure, err := resolveSolanaTransferFeeStructure(ctx, chainID, feeToken, mintAddresses, feeStructureOverride, solanaParams)
	if err != nil {
		return "", err
	}
	recipientAmount := new(big.Int).Neg(amountChanges[0])
	totalFee := fees.CalculateTotalFee(recipientAmount, feeStructure)
	amountChanges[0] = new(big.Int).Sub(amountChanges[0], totalFee)

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	inputUtxos, outputUtxos, err := getSolanaTransferInputAndOutputUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, recipientAddress, recipientAmount)
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
	encryptedOutputBytes, encryptedOutputInts, err := solanaTransferEncryptedOutputs(outputUtxos)
	if err != nil {
		return "", err
	}
	inputUtxosArray := [][]*utxo.Utxo{inputUtxos}
	if err := snarkjs.EnsureAmountChanges(inputUtxosArray, [][]*utxo.Utxo{{outputUtxos[0]}}, amountChanges); err != nil {
		return "", err
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
		OutputUtxos:           [][]*utxo.Utxo{outputUtxos},
		ExtraRandomization:    extraRandomization,
		RelayerFee:            totalFee,
		VariableRate:          big.NewInt(0),
		RecipientAddress:      constants.SolanaNativeAddress,
		SignerAddress:         relay,
		Dimensions:            dimensions,
		EncryptedOutputs:      encryptedOutputBytes,
		ChainID:               chainID,
	})
	if err != nil {
		return "", err
	}

	return web3.SolanaTransactCallRelayer(ctx, api.SolanaTransactionBody{
		ChainID:      chainID,
		RelayAddress: relay,
		FunctionName: "transfer",
		Args: api.SolanaArgs{
			ProofAArr:        proof.ProofAArr,
			ProofBArr:        proof.ProofBArr,
			ProofCArr:        proof.ProofCArr,
			PublicInputsArr:  proof.PublicInputsArr,
			EncryptedOutputs: encryptedOutputInts,
			RelayerFee:       totalFee.String(),
			Dimensions:       dimensions,
		},
		Accounts: api.SolanaTransactAccounts{
			Recipient: relay,
			Mint:      nonNativeMintString(mintAddresses[0]),
		},
		CommitmentValidationData: proof.CommitmentValidationData,
	})
}

func nonNativeMintString(mintAddress string) string {
	if mintAddress == constants.SolanaNativeAddress {
		return ""
	}
	return mintAddress
}
