package transactions

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"

	bin "github.com/gagliardetto/binary"
	solana "github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	tokenprogram "github.com/gagliardetto/solana-go/programs/token"
	token2022 "github.com/gagliardetto/solana-go/programs/token-2022"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	errorhandling "github.com/gioeba/go_sdk_test/error-handling"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/types"
)

type multiPaymentDepositArgs struct {
	Amounts                  []uint64
	StealthAddressStructures []pretransaction.AnchorStealthAddressStructure
	CreateBlockedUtxos       bool
}

var (
	errAmountsEmpty            = errors.New("transactions: amounts must not be empty")
	errAmountsStealthMismatch  = errors.New("transactions: amounts and stealthAddressStructures length mismatch")
	errAmountNotPositive       = errors.New("transactions: all amounts must be positive")
	errSolanaAmountTooLarge    = errors.New("transactions: Solana amount does not fit in uint64")
	errNotSolanaChain          = errors.New("transactions: Solana deposit requires a Solana chain")
	errSolanaProoflessOneMint  = errors.New("transactions: Solana prooflessDeposit supports one mint per transaction")
	multiPaymentDepositDiscrim = []byte{70, 64, 60, 132, 3, 67, 69, 74}
)

const solanaBlockheightExpiredRetryCount = 10

func validateSolanaDepositArgs(amounts []*big.Int, structures []types.StealthAddressStructure) error {
	if len(amounts) == 0 {
		return errAmountsEmpty
	}
	if len(amounts) != len(structures) {
		return errAmountsStealthMismatch
	}
	for _, amount := range amounts {
		if amount.Sign() <= 0 {
			return errAmountNotPositive
		}
	}
	return assertNoDuplicateStealthAddressStructures(structures)
}

func encodeMultiPaymentDepositData(amounts []*big.Int, structures []types.StealthAddressStructure, createBlockedUtxos bool) ([]byte, error) {
	args := multiPaymentDepositArgs{
		Amounts:                  make([]uint64, len(amounts)),
		StealthAddressStructures: make([]pretransaction.AnchorStealthAddressStructure, len(structures)),
		CreateBlockedUtxos:       createBlockedUtxos,
	}
	for i, amount := range amounts {
		args.Amounts[i] = amount.Uint64()
	}
	for i, s := range structures {
		args.StealthAddressStructures[i] = pretransaction.BuildAnchorStealthAddressStructure(s)
	}

	body, err := bin.MarshalBorsh(args)
	if err != nil {
		return nil, err
	}
	return append(append([]byte{}, multiPaymentDepositDiscrim...), body...), nil
}

func buildMultiPaymentDepositInstruction(
	programID, signer, originalDeployer solana.PublicKey,
	mintAddress string,
	amounts []*big.Int,
	structures []types.StealthAddressStructure,
	createBlockedUtxos bool,
) (*solana.GenericInstruction, error) {
	storageAccount, err := pretransaction.GetStorageAccountPublicKey(programID, originalDeployer)
	if err != nil {
		return nil, err
	}
	storageVault, err := pretransaction.GetStorageVaultPublicKey(programID, originalDeployer)
	if err != nil {
		return nil, err
	}
	merkleAccount, err := pretransaction.GetMerkleAccountPublicKey(programID, originalDeployer)
	if err != nil {
		return nil, err
	}

	isNative := mintAddress == constants.SolanaNativeAddress
	mintMeta := solana.NewAccountMeta(programID, false, false)
	signerAtaMeta := solana.NewAccountMeta(programID, false, false)
	storageVaultAtaMeta := solana.NewAccountMeta(programID, false, false)
	if !isNative {
		mint, err := solana.PublicKeyFromBase58(mintAddress)
		if err != nil {
			return nil, err
		}
		signerAta, _, err := solana.FindAssociatedTokenAddress(signer, mint)
		if err != nil {
			return nil, err
		}
		storageVaultAta, _, err := solana.FindAssociatedTokenAddress(storageVault, mint)
		if err != nil {
			return nil, err
		}
		mintMeta = solana.NewAccountMeta(mint, false, false)
		signerAtaMeta = solana.NewAccountMeta(signerAta, true, false)
		storageVaultAtaMeta = solana.NewAccountMeta(storageVaultAta, true, false)
	}

	accounts := solana.AccountMetaSlice{
		solana.NewAccountMeta(signer, true, true),
		solana.NewAccountMeta(originalDeployer, false, false),
		solana.NewAccountMeta(storageAccount, true, false),
		solana.NewAccountMeta(storageVault, true, false),
		mintMeta,
		signerAtaMeta,
		storageVaultAtaMeta,
		solana.NewAccountMeta(merkleAccount, true, false),
		solana.NewAccountMeta(solana.TokenProgramID, false, false),
		solana.NewAccountMeta(solana.SPLAssociatedTokenAccountProgramID, false, false),
		solana.NewAccountMeta(solana.SystemProgramID, false, false),
	}

	data, err := encodeMultiPaymentDepositData(amounts, structures, createBlockedUtxos)
	if err != nil {
		return nil, err
	}
	return solana.NewInstruction(programID, accounts, data), nil
}

func solanaAmountUint64(amount *big.Int) (uint64, error) {
	if amount == nil || amount.Sign() < 0 {
		return 0, errAmountNotPositive
	}
	if !amount.IsUint64() {
		return 0, errSolanaAmountTooLarge
	}
	return amount.Uint64(), nil
}

func buildSolanaTransferInstructions(
	from solana.PublicKey,
	to solana.PublicKey,
	token types.ERC20Token,
	amount *big.Int,
) ([]solana.Instruction, error) {
	value, err := solanaAmountUint64(amount)
	if err != nil {
		return nil, err
	}
	if value == 0 {
		return nil, nil
	}

	if token.Erc20TokenAddress == constants.SolanaNativeAddress {
		return []solana.Instruction{
			system.NewTransferInstruction(value, from, to).Build(),
		}, nil
	}

	mint, err := solana.PublicKeyFromBase58(token.Erc20TokenAddress)
	if err != nil {
		return nil, err
	}

	tokenProgramID := solana.TokenProgramID
	if token.Is2022Program {
		tokenProgramID = solana.Token2022ProgramID
	}
	fromATA, _, err := solana.FindAssociatedTokenAddressWithProgram(from, mint, tokenProgramID)
	if err != nil {
		return nil, err
	}
	toATA, _, err := solana.FindAssociatedTokenAddressWithProgram(to, mint, tokenProgramID)
	if err != nil {
		return nil, err
	}

	instructions := []solana.Instruction{
		associatedtokenaccount.NewCreateIdempotentInstructionWithTokenProgram(from, to, mint, tokenProgramID).Build(),
	}
	if token.Is2022Program {
		if token.Decimals < 0 || token.Decimals > 255 {
			return nil, fmt.Errorf("transactions: invalid Solana token decimals %d", token.Decimals)
		}
		instructions = append(instructions, token2022.NewTransferCheckedInstruction(
			value,
			uint8(token.Decimals),
			fromATA,
			mint,
			toATA,
			from,
			nil,
		).Build())
	} else {
		instructions = append(instructions, tokenprogram.NewTransferInstruction(value, fromATA, toATA, from, nil).Build())
	}
	return instructions, nil
}

func buildSolanaDepositTransaction(
	ctx context.Context,
	connection *rpc.Client,
	signer solana.PublicKey,
	instructions []solana.Instruction,
) (*solana.Transaction, error) {
	latest, err := connection.GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return nil, err
	}
	return solana.NewTransaction(
		instructions,
		latest.Value.Blockhash,
		solana.TransactionPayer(signer),
	)
}

func shouldRetrySolanaSendError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "blockhash not found") ||
		strings.Contains(msg, "blockhashnotfound") ||
		strings.Contains(msg, "transactionexpiredblockheightexceedederror") ||
		strings.Contains(msg, "block height exceeded")
}

func signAndSendSolanaInstructions(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	programID solana.PublicKey,
	connection *rpc.Client,
	signer solana.PublicKey,
	instructions []solana.Instruction,
) (string, error) {
	program, err := hinkal.GetSolanaProgram(programID)
	if err != nil {
		return "", err
	}

	for i := 0; i < solanaBlockheightExpiredRetryCount; i++ {
		tx, err := buildSolanaDepositTransaction(ctx, connection, signer, instructions)
		if err != nil {
			return "", err
		}

		signature, err := program.SignAndSend(ctx, tx)
		if err == nil {
			return signature.String(), nil
		}
		if i == solanaBlockheightExpiredRetryCount-1 || !shouldRetrySolanaSendError(err) {
			return "", err
		}
	}
	return "", errors.New("transactions: Solana deposit failed")
}

func SubmitSolanaProoflessDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	token types.ERC20Token,
	amounts []*big.Int,
	structures []types.StealthAddressStructure,
	returnTxData bool,
) (string, error) {
	if err := validateSolanaDepositArgs(amounts, structures); err != nil {
		return "", err
	}
	chainID, err := pretransaction.ValidateAndGetChainID([]types.ERC20Token{token})
	if err != nil {
		return "", err
	}
	if !constants.IsSolanaLike(chainID) {
		return "", errNotSolanaChain
	}

	programID, err := solana.PublicKeyFromBase58(hinkal.HinkalAddress(chainID))
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
	signer, err := hinkal.GetSolanaPublicKey(ctx)
	if err != nil {
		return "", err
	}
	connection, err := hinkal.GetSolanaConnection()
	if err != nil {
		return "", err
	}

	instruction, err := buildMultiPaymentDepositInstruction(programID, signer, originalDeployer, token.Erc20TokenAddress, amounts, structures, false)
	if err != nil {
		return "", err
	}

	if returnTxData {
		tx, err := buildSolanaDepositTransaction(ctx, connection, signer, []solana.Instruction{instruction})
		if err != nil {
			return "", err
		}
		raw, err := tx.MarshalBinary()
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(raw), nil
	}

	return signAndSendSolanaInstructions(ctx, hinkal, programID, connection, signer, []solana.Instruction{instruction})
}

func HinkalSolanaProoflessDepositWithPublicFee(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	token types.ERC20Token,
	amounts []*big.Int,
	structures []types.StealthAddressStructure,
	feeAmount *big.Int,
) (string, error) {
	if err := validateSolanaDepositArgs(amounts, structures); err != nil {
		return "", err
	}
	chainID, err := pretransaction.ValidateAndGetChainID([]types.ERC20Token{token})
	if err != nil {
		return "", err
	}
	if !constants.IsSolanaLike(chainID) {
		return "", errNotSolanaChain
	}

	programID, err := solana.PublicKeyFromBase58(hinkal.HinkalAddress(chainID))
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
	signer, err := hinkal.GetSolanaPublicKey(ctx)
	if err != nil {
		return "", err
	}
	connection, err := hinkal.GetSolanaConnection()
	if err != nil {
		return "", err
	}
	feeRecipientStr, err := hinkal.GetRandomRelay(ctx, chainID, false)
	if err != nil {
		return "", err
	}
	if feeRecipientStr == "" {
		return "", fmt.Errorf("transactions: no relay available for chain %d", chainID)
	}
	feeRecipient, err := solana.PublicKeyFromBase58(feeRecipientStr)
	if err != nil {
		return "", err
	}

	feeInstructions, err := buildSolanaTransferInstructions(signer, feeRecipient, token, feeAmount)
	if err != nil {
		return "", err
	}
	depositInstruction, err := buildMultiPaymentDepositInstruction(programID, signer, originalDeployer, token.Erc20TokenAddress, amounts, structures, false)
	if err != nil {
		return "", err
	}
	instructions := append(feeInstructions, depositInstruction)
	return signAndSendSolanaInstructions(ctx, hinkal, programID, connection, signer, instructions)
}

func HinkalSolanaProoflessDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChanges []*big.Int,
	stealthAddressStructuresOverride []types.StealthAddressStructure,
	returnTxData bool,
) (string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return "", err
	}
	if !constants.IsSolanaLike(chainID) {
		return "", errNotSolanaChain
	}
	if len(erc20Tokens) != len(amountChanges) {
		return "", errTokenAmountLengthMismatch
	}

	firstToken := erc20Tokens[0]
	for _, token := range erc20Tokens {
		if token.Erc20TokenAddress != firstToken.Erc20TokenAddress {
			return "", errSolanaProoflessOneMint
		}
	}

	stealthAddressStructures, err := getProoflessStealthAddressStructures(hinkal, len(amountChanges), stealthAddressStructuresOverride)
	if err != nil {
		return "", err
	}

	return SubmitSolanaProoflessDeposit(ctx, hinkal, firstToken, amountChanges, stealthAddressStructures, returnTxData)
}

func HinkalSolanaDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	amount *big.Int,
	token types.ERC20Token,
	returnTxData bool,
) (string, error) {
	userKeys := hinkal.GetUserKeys()
	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
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
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return "", err
	}
	spendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}
	structure, err := snarkjs.CalcStealthAddressStructure(extraRandomization, shieldedPrivateKey, spendingPublicKey)
	if err != nil {
		return "", err
	}
	return SubmitSolanaProoflessDeposit(ctx, hinkal, token, []*big.Int{amount}, []types.StealthAddressStructure{structure}, returnTxData)
}

func HinkalSolanaDepositForOther(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	amount *big.Int,
	token types.ERC20Token,
	recipientInfo string,
	returnTxData bool,
) (string, error) {
	if !pretransaction.IsValidPrivateAddress(recipientInfo) {
		return "", errorhandling.ErrRecipientFormatIncorrect
	}
	structure, err := pretransaction.ConstructStealthAddressStructure(recipientInfo)
	if err != nil {
		return "", err
	}
	return SubmitSolanaProoflessDeposit(ctx, hinkal, token, []*big.Int{amount}, []types.StealthAddressStructure{structure}, returnTxData)
}
