package transactions

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	errorhandling "github.com/gioeba/go_sdk_test/error-handling"
	"github.com/gioeba/go_sdk_test/internal/functions/fees"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	privatewallet "github.com/gioeba/go_sdk_test/internal/functions/private-wallet"
	"github.com/gioeba/go_sdk_test/internal/functions/snarkjs"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

var (
	errDepositAndBridgeNoRecipients      = errors.New("transactions: depositAndBridge recipients must not be empty")
	errDepositAndBridgeUnsupportedSource = errors.New("transactions: depositAndBridge supports EVM source chains only")
	errDepositAndBridgeMissingNativeUtxo = errors.New("transactions: native fee UTXO is required for Lifi bridge")
)

type bridgeRecipientUtxo struct {
	types.BridgeRecipient
	utxo       *utxo.Utxo
	nativeUtxo *utxo.Utxo
}

func nativeFeeValue(quote types.BridgeQuote) *big.Int {
	if quote.NativeFee == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(quote.NativeFee)
}

func validateDepositAndBridgeArgs(erc20Tokens []types.ERC20Token, recipients []types.BridgeRecipient) error {
	if len(erc20Tokens) == 0 {
		return errDepositAndWithdrawNoToken
	}
	if len(erc20Tokens) > 1 {
		return errDepositAndWithdrawOneToken
	}
	if len(recipients) == 0 {
		return errDepositAndBridgeNoRecipients
	}
	for _, recipient := range recipients {
		if recipient.BridgeAmount == nil || recipient.BridgeAmount.Sign() <= 0 {
			return errAmountNotPositive
		}
		if recipient.RecipientAddress == "" || recipient.TemporarySubAccount.EthAddress == "" || recipient.TemporarySubAccount.PrivateKey == "" {
			return errorhandling.ErrRecipientFormatIncorrect
		}
		if recipient.Quote.Calldata == "" {
			return errors.New("transactions: bridge quote calldata is required")
		}
	}
	return nil
}

func resolveDepositAndBridgeFeeStructure(
	ctx context.Context,
	chainID int,
	tokenAddress string,
	recipients []types.BridgeRecipient,
	feeStructureOverride *types.FeeStructure,
) (types.FeeStructure, error) {
	if feeStructureOverride != nil {
		return paySendFeeStructure(*feeStructureOverride), nil
	}
	lifiRouterAddress, err := constants.LifiRouterAddress(chainID)
	if err != nil {
		return types.FeeStructure{}, err
	}
	firstRecipient := recipients[0]
	sampleOps, err := privatewallet.CreateLifiBridgeOps(
		chainID,
		firstRecipient.TemporarySubAccount.EthAddress,
		lifiRouterAddress,
		tokenAddress,
		firstRecipient.BridgeAmount,
		firstRecipient.BridgeAmount,
		firstRecipient.Quote,
	)
	if err != nil {
		return types.FeeStructure{}, err
	}
	sampleCalls := make([]types.CallInfo, len(sampleOps))
	for i, op := range sampleOps {
		call, err := privatewallet.ConvertEmporiumOpToCallInfo(op, firstRecipient.TemporarySubAccount.EthAddress, chainID)
		if err != nil {
			return types.FeeStructure{}, err
		}
		sampleCalls[i] = call
	}
	feeStructure, err := pretransaction.GetFeeStructure(
		ctx,
		chainID,
		tokenAddress,
		[]string{tokenAddress},
		types.ExternalActionEmporium,
		sampleCalls,
		big.NewInt(constants.PaySendVariableRate),
		nil,
	)
	if err != nil {
		return types.FeeStructure{}, err
	}
	return paySendFeeStructure(feeStructure), nil
}

func zeroNativeDepositFeeStructure(feeStructure types.FeeStructure) types.FeeStructure {
	feeToken := feeStructure.FeeToken
	if feeToken == "" {
		feeToken = constants.DefaultFeeToken
	}
	return types.FeeStructure{
		FeeToken:     feeToken,
		FlatFee:      big.NewInt(0),
		VariableRate: big.NewInt(0),
	}
}

func depositedBridgeRecipients(recipients []types.BridgeRecipient, mainDeposits, nativeDeposits []recipientUtxo) []bridgeRecipientUtxo {
	out := make([]bridgeRecipientUtxo, len(recipients))
	for i, recipient := range recipients {
		out[i] = bridgeRecipientUtxo{
			BridgeRecipient: recipient,
			utxo:            mainDeposits[i].utxo,
		}
		if len(nativeDeposits) > i {
			out[i].nativeUtxo = nativeDeposits[i].utxo
		}
	}
	return out
}

func bridgeInputUtxoForToken(source *utxo.Utxo, tokenAddress string) (*utxo.Utxo, error) {
	if strings.EqualFold(source.Erc20TokenAddress, tokenAddress) {
		return source, nil
	}
	return utxo.CreateFrom(source, types.UtxoParams{Erc20TokenAddress: tokenAddress})
}

func hinkalBridgeBatch(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	recipients []bridgeRecipientUtxo,
	feeStructure types.FeeStructure,
	hashedEthereumAddress string,
	statusID string,
	txCompletionTime *int,
) (string, error) {
	if len(recipients) == 0 {
		return "", errDepositAndBridgeNoRecipients
	}
	tokenAddress := erc20Token.Erc20TokenAddress
	contractData, err := constants.GetContractData(chainID)
	if err != nil {
		return "", err
	}
	if contractData.EmporiumAddress == "" {
		return "", errors.New("transactions: emporium address not set")
	}
	lifiRouterAddress, err := constants.LifiRouterAddress(chainID)
	if err != nil {
		return "", err
	}
	relay, err := relayerAddress(ctx, hinkal, chainID)
	if err != nil {
		return "", err
	}
	fetchClient, err := hinkal.GetFetchClient(chainID)
	if err != nil {
		return "", err
	}

	timeStamp := new(big.Int).SetInt64(utils.GetCurrentTimeInSeconds()).String()
	transactions := make([]web3.TransactCallRelayerBatchItem, 0, len(recipients))
	for _, recipient := range recipients {
		if _, err := api.AddTemporaryWalletNonce(ctx, chainID, hashedEthereumAddress, recipient.TemporarySubAccount.Index); err != nil {
			return "", err
		}

		utxoToBridge, err := bridgeInputUtxoForToken(recipient.utxo, tokenAddress)
		if err != nil {
			return "", err
		}
		zeroUtxo, err := buildDepositAndWithdrawZeroUtxo(hinkal, tokenAddress, utxoToBridge, timeStamp)
		if err != nil {
			return "", err
		}

		inputUtxosArray := [][]*utxo.Utxo{{utxoToBridge, zeroUtxo}}
		outputUtxosArray := [][]*utxo.Utxo{{zeroUtxo}}
		onChainCreation := []bool{false}

		needsNativeFee := nativeFeeValue(recipient.Quote).Sign() > 0 && !strings.EqualFold(tokenAddress, constants.ZeroAddress)
		if needsNativeFee {
			if recipient.nativeUtxo == nil {
				return "", errDepositAndBridgeMissingNativeUtxo
			}
			nativeUtxo, err := bridgeInputUtxoForToken(recipient.nativeUtxo, constants.ZeroAddress)
			if err != nil {
				return "", err
			}
			nativeZeroUtxo, err := buildDepositAndWithdrawZeroUtxo(hinkal, constants.ZeroAddress, nativeUtxo, timeStamp)
			if err != nil {
				return "", err
			}
			inputUtxosArray = append(inputUtxosArray, []*utxo.Utxo{nativeUtxo, nativeZeroUtxo})
			outputUtxosArray = append(outputUtxosArray, []*utxo.Utxo{nativeZeroUtxo})
			onChainCreation = append(onChainCreation, false)
		}

		ops, err := privatewallet.CreateLifiBridgeOps(
			chainID,
			recipient.TemporarySubAccount.EthAddress,
			lifiRouterAddress,
			tokenAddress,
			utxoToBridge.Amount,
			recipient.BridgeAmount,
			recipient.Quote,
		)
		if err != nil {
			return "", err
		}
		bridgeFeeStructure := fees.CalculateModifiedFeeStructure(ctx, chainID, erc20Token, recipient.BridgeAmount, feeStructure)
		proof, err := snarkjs.ConstructZkProof(ctx, snarkjs.ConstructZkProofParams{
			MerkleTree:             hinkal.MerkleTree(chainID),
			InputUtxos:             inputUtxosArray,
			OutputUtxos:            outputUtxosArray,
			UserKeys:               hinkal.GetUserKeys(),
			ExternalActionID:       types.ExternalActionEmporium,
			ExternalAddress:        contractData.EmporiumAddress,
			ExternalActionMetadata: ops,
			GenerateProofRemotely:  hinkal.GenerateProofRemotely(),
			FeeStructure:           bridgeFeeStructure,
			Relay:                  relay,
			ChainID:                chainID,
			OnChainCreation:        onChainCreation,
			SubAccountPrivateKey:   recipient.TemporarySubAccount.PrivateKey,
		})
		if err != nil {
			return "", err
		}
		authorizationData, err := privatewallet.GetAuthorizationDataIfNeeded(ctx, fetchClient, chainID, recipient.TemporarySubAccount.PrivateKey)
		if err != nil {
			return "", err
		}

		transactions = append(transactions, web3.TransactCallRelayerBatchItem{
			ZkCallData:               proof.ZkCallData,
			DimData:                  proof.DimData,
			CircomData:               proof.CircomData,
			CommitmentValidationData: proof.CommitmentValidationData,
			AuthorizationData:        authorizationData,
			RecipientAddress:         recipient.RecipientAddress,
		})
	}

	_, _ = api.SafeUpdateDepositAndWithdrawStatus(ctx, api.UpdateDepositAndWithdrawStatusRequestBody{
		ID:                    statusID,
		ChainID:               chainID,
		HashedEthereumAddress: hashedEthereumAddress,
		Phase:                 types.DepositAndWithdrawPhaseBeforeScheduleWithdraw,
	})
	scheduleID, err := web3.TransactCallRelayerBatch(ctx, chainID, transactions, hashedEthereumAddress, txCompletionTime, "", "")
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

func HinkalDepositAndBridge(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	recipients []types.BridgeRecipient,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
	preEstimateGas bool,
) (types.DepositAndSendExtendedResult, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if constants.IsSolanaLike(chainID) || constants.IsTronLike(chainID) {
		return types.DepositAndSendExtendedResult{}, errDepositAndBridgeUnsupportedSource
	}
	if err := validateDepositAndBridgeArgs(erc20Tokens, recipients); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	erc20Token := erc20Tokens[0]
	tokenAddress := erc20Token.Erc20TokenAddress
	rawEthereumAddress, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	ethereumAddress, err := utils.AddressToHexFormat(rawEthereumAddress)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	hashedEthereumAddress := utils.HashEthereumAddress(ethereumAddress)

	feeStructure, err := resolveDepositAndBridgeFeeStructure(ctx, chainID, tokenAddress, recipients, feeStructureOverride)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	recipientAddresses := make([]string, len(recipients))
	bridgeAmounts := make([]*big.Int, len(recipients))
	nativeFeeAmounts := make([]*big.Int, len(recipients))
	totalNativeFee := big.NewInt(0)
	for i, recipient := range recipients {
		recipientAddresses[i] = recipient.RecipientAddress
		bridgeAmounts[i] = recipient.BridgeAmount
		nativeFeeAmounts[i] = nativeFeeValue(recipient.Quote)
		totalNativeFee.Add(totalNativeFee, nativeFeeAmounts[i])
	}

	mainDeposits, statusID, depositTxHash, err := HinkalDepositOnChainUtxos(
		ctx,
		hinkal,
		chainID,
		erc20Token,
		bridgeAmounts,
		recipientAddresses,
		feeStructure,
		hashedEthereumAddress,
		preEstimateGas,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	var nativeDeposits []recipientUtxo
	needsNativeDeposit := totalNativeFee.Sign() > 0 && !strings.EqualFold(tokenAddress, constants.ZeroAddress)
	if needsNativeDeposit {
		nativeToken := constants.GetERC20Token(constants.ZeroAddress, chainID)
		if nativeToken == nil {
			return types.DepositAndSendExtendedResult{}, errors.New("transactions: native token not found")
		}
		nativeDeposits, _, _, err = HinkalDepositOnChainUtxos(
			ctx,
			hinkal,
			chainID,
			*nativeToken,
			nativeFeeAmounts,
			recipientAddresses,
			zeroNativeDepositFeeStructure(feeStructure),
			hashedEthereumAddress,
			preEstimateGas,
		)
		if err != nil {
			return types.DepositAndSendExtendedResult{}, err
		}
	}

	allDeposits := append([]recipientUtxo{}, mainDeposits...)
	allDeposits = append(allDeposits, nativeDeposits...)
	if err := waitForDepositedUtxosInMerkleTree(ctx, hinkal, chainID, allDeposits); err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}

	scheduleID, err := hinkalBridgeBatch(
		ctx,
		hinkal,
		chainID,
		erc20Token,
		depositedBridgeRecipients(recipients, mainDeposits, nativeDeposits),
		feeStructure,
		hashedEthereumAddress,
		statusID,
		txCompletionTime,
	)
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	return types.DepositAndSendExtendedResult{DepositTxHash: depositTxHash, ScheduleID: scheduleID}, nil
}
