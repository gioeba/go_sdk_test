package transactions

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	"github.com/gioeba/go_sdk_test/internal/functions/balance"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

var (
	errSolanaWithdrawOneMint       = errors.New("transactions: Solana withdraw supports one mint per transaction")
	errSolanaWithdrawMissingOutput = errors.New("transactions: Solana withdraw missing output UTXO")
)

func getSolanaWithdrawInputAndOutputUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
) ([]*utxo.Utxo, []*utxo.Utxo, error) {
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, 6, nil, false, false)
	if err != nil {
		return nil, nil, err
	}
	userKeys := hinkal.GetUserKeys()
	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	outputUtxos, err := pretransaction.OutputUtxoProcessing(userKeys, inputUtxosArray[0], amountChanges[0], timeStamp, true, "", nil)
	if err != nil {
		return nil, nil, err
	}
	return inputUtxosArray[0], outputUtxos, nil
}

func solanaEncryptedOutputBytes(outputUtxos []*utxo.Utxo) ([][]byte, [][]int, error) {
	encryptedOutputs, err := snarkjs.CalcEncryptedOutputs([][]*utxo.Utxo{outputUtxos})
	if err != nil {
		return nil, nil, err
	}
	if len(encryptedOutputs) == 0 || len(encryptedOutputs[0]) == 0 {
		return nil, nil, errSolanaWithdrawMissingOutput
	}
	encryptedOutputBytes := common.FromHex(encryptedOutputs[0][0])
	bytes := [][]byte{encryptedOutputBytes}
	ints := make([][]int, len(bytes))
	for i, row := range bytes {
		ints[i] = make([]int, len(row))
		for j, b := range row {
			ints[i][j] = int(b)
		}
	}
	return bytes, ints, nil
}

func HinkalSolanaWithdraw(
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
		return "", errWithdrawNoToken
	}
	if len(erc20Tokens) > 1 {
		return "", errSolanaWithdrawOneMint
	}

	amountChanges := copyBigInts(amountChangesBase)
	mintAddresses := tokenAddresses(erc20Tokens)
	token := erc20Tokens[0]
	if feeToken == "" {
		feeToken = mintAddresses[0]
	}
	solanaParams := &api.SolanaGasEstimateParams{
		MintTo:         mintAddresses[0],
		Recipient:      recipientAddress,
		NullifierCount: pretransaction.CalculateSolanaNullifierCount(ctx, hinkal, chainID, mintAddresses, amountChanges),
	}
	feeStructure, err := resolveWithdrawFeeStructure(ctx, chainID, feeToken, mintAddresses, token, amountChanges[0], feeStructureOverride, solanaParams)
	if err != nil {
		return "", err
	}
	amountChanges[0] = new(big.Int).Sub(amountChanges[0], feeStructure.FlatFee)

	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	inputUtxos, outputUtxos, err := getSolanaWithdrawInputAndOutputUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges)
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
	encryptedOutputBytes, encryptedOutputInts, err := solanaEncryptedOutputBytes(outputUtxos)
	if err != nil {
		return "", err
	}
	inputUtxosArray := [][]*utxo.Utxo{inputUtxos}
	outputUtxosArray := [][]*utxo.Utxo{outputUtxos}
	if err := snarkjs.EnsureAmountChanges(inputUtxosArray, outputUtxosArray, amountChanges); err != nil {
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
		return "", err
	}

	accounts := api.SolanaTransactAccounts{Recipient: recipientAddress}
	if !strings.EqualFold(mintAddresses[0], constants.SolanaNativeAddress) {
		accounts.Mint = mintAddresses[0]
	}

	return web3.SolanaTransactCallRelayer(ctx, api.SolanaTransactionBody{
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
}
