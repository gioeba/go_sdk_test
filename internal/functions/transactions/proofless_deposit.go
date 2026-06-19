package transactions

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/contractabi"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/tron"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
)

var (
	errProoflessNotImplemented   = errors.New("transactions: prooflessDeposit is not implemented for this chain")
	errTokenAmountLengthMismatch = errors.New("transactions: erc20 tokens and amountChanges length mismatch")
	errStealthLengthMismatch     = errors.New("transactions: stealth address structures length mismatch")
	errDuplicateStealth          = errors.New("transactions: duplicate randomization and stealth address pair detected in stealthAddressStructures")
)

type tokenWithBalance struct {
	token   types.ERC20Token
	balance *big.Int
}

func positiveAmount(amount *big.Int) *big.Int {
	if amount != nil && amount.Sign() > 0 {
		return new(big.Int).Set(amount)
	}
	return big.NewInt(0)
}

func aggregateAmountsForApproval(erc20Tokens []types.ERC20Token, amounts []*big.Int) []tokenWithBalance {
	out := make([]tokenWithBalance, 0, len(erc20Tokens))
	for i, token := range erc20Tokens {
		found := false
		for j := range out {
			if out[j].token.ChainID == token.ChainID && strings.EqualFold(out[j].token.Erc20TokenAddress, token.Erc20TokenAddress) {
				out[j].balance = new(big.Int).Add(out[j].balance, amounts[i])
				found = true
				break
			}
		}
		if !found {
			out = append(out, tokenWithBalance{token: token, balance: new(big.Int).Set(amounts[i])})
		}
	}
	return out
}

func assertNoDuplicateStealthAddressStructures(structures []types.StealthAddressStructure) error {
	seen := make(map[string]struct{}, len(structures))
	for _, s := range structures {
		key := fmt.Sprintf("%s:%s", s.ExtraRandomization.String(), s.StealthAddress.String())
		if _, ok := seen[key]; ok {
			return errDuplicateStealth
		}
		seen[key] = struct{}{}
	}
	return nil
}

func assertWrapperArgLengths(fnName string, erc20Tokens []types.ERC20Token, amounts []*big.Int, structures []types.StealthAddressStructure) error {
	if len(erc20Tokens) != len(amounts) || len(amounts) != len(structures) {
		return fmt.Errorf("%s: array length mismatch", fnName)
	}
	return nil
}

func buildWrapperApprovalInputs(
	feeToken string,
	feeAmount *big.Int,
	erc20Tokens []types.ERC20Token,
	amounts []*big.Int,
) ([]tokenWithBalance, *big.Int) {
	feeAmount = positiveAmount(feeAmount)
	isFeeNative := strings.EqualFold(feeToken, constants.ZeroAddress)

	ethValue := big.NewInt(0)
	for i, token := range erc20Tokens {
		if strings.EqualFold(token.Erc20TokenAddress, constants.ZeroAddress) {
			ethValue.Add(ethValue, positiveAmount(amounts[i]))
		}
	}
	if isFeeNative {
		ethValue.Add(ethValue, feeAmount)
	}

	depositByToken := aggregateAmountsForApproval(erc20Tokens, amounts)
	tokensForApproval := make([]tokenWithBalance, 0, len(depositByToken)+1)
	feeTokenInDeposit := false
	for _, twb := range depositByToken {
		if strings.EqualFold(twb.token.Erc20TokenAddress, constants.ZeroAddress) {
			continue
		}
		balance := new(big.Int).Set(positiveAmount(twb.balance))
		if !isFeeNative && strings.EqualFold(twb.token.Erc20TokenAddress, feeToken) {
			balance.Add(balance, feeAmount)
			feeTokenInDeposit = true
		}
		if balance.Sign() > 0 {
			tokensForApproval = append(tokensForApproval, tokenWithBalance{
				token:   twb.token,
				balance: balance,
			})
		}
	}

	if !isFeeNative && feeAmount.Sign() > 0 && !feeTokenInDeposit {
		chainID := 0
		if len(erc20Tokens) > 0 {
			chainID = erc20Tokens[0].ChainID
		}
		tokensForApproval = append(tokensForApproval, tokenWithBalance{
			token:   types.ERC20Token{ChainID: chainID, Erc20TokenAddress: feeToken},
			balance: feeAmount,
		})
	}

	return tokensForApproval, ethValue
}

func tronApprovalsFromTokenBalances(tokensWithBalances []tokenWithBalance) ([]tron.TokenApprovalAmount, error) {
	approvals := make([]tron.TokenApprovalAmount, 0, len(tokensWithBalances))
	for _, twb := range tokensWithBalances {
		if strings.EqualFold(twb.token.Erc20TokenAddress, constants.ZeroAddress) || twb.balance.Sign() <= 0 {
			continue
		}
		tokenHex, err := utils.AddressToHexFormat(twb.token.Erc20TokenAddress)
		if err != nil {
			return nil, err
		}
		tokenBase58, err := utils.EVMHexToTronBase58Address(tokenHex)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, tron.TokenApprovalAmount{
			TokenBase58: tokenBase58,
			TokenHex:    tokenHex,
			Amount:      new(big.Int).Set(twb.balance),
		})
	}
	return approvals, nil
}

func getProoflessStealthAddressStructures(
	hinkal ihinkal.HinkalInternal,
	count int,
	overrides []types.StealthAddressStructure,
) ([]types.StealthAddressStructure, error) {
	if overrides != nil {
		if len(overrides) != count {
			return nil, errStealthLengthMismatch
		}
		return overrides, nil
	}

	structures := make([]types.StealthAddressStructure, count)
	for i := range structures {
		structure, err := pretransaction.GetStealthAddressStructureFromUserKeys(hinkal.GetUserKeys())
		if err != nil {
			return nil, err
		}
		structures[i] = structure
	}
	return structures, nil
}

func hinkalTronProoflessDeposit(
	ctx context.Context,
	client tron.TronClient,
	chainID int,
	erc20Tokens []types.ERC20Token,
	amountChanges []*big.Int,
	stealthAddressStructures []types.StealthAddressStructure,
) (string, error) {
	grpc := client.GrpcClient()
	ownerBase58 := client.GetAddress()
	ownerHex, err := utils.AddressToHexFormat(ownerBase58)
	if err != nil {
		return "", err
	}

	hinkalHex, err := constants.HinkalAddress(chainID)
	if err != nil {
		return "", err
	}
	hinkalBase58, err := utils.EVMHexToTronBase58Address(hinkalHex)
	if err != nil {
		return "", err
	}

	erc20Addresses, err := tron.NormalizedTokenAddresses(erc20Tokens)
	if err != nil {
		return "", err
	}
	tokenIDs := make([]*big.Int, len(erc20Tokens))
	for i := range tokenIDs {
		tokenIDs[i] = big.NewInt(0)
	}

	callData, err := contractabi.PackProoflessDeposit(chainID, erc20Addresses, amountChanges, tokenIDs, stealthAddressStructures)
	if err != nil {
		return "", err
	}

	callValueSun, approvals, err := tron.SplitTokensByNative(erc20Tokens, amountChanges)
	if err != nil {
		return "", err
	}

	if err := tron.ApproveTokens(ctx, client, chainID, ownerBase58, ownerHex, hinkalBase58, approvals); err != nil {
		return "", err
	}

	if err := tron.SimulateTronTransaction(ctx, chainID, hinkalBase58, ownerBase58, callData, callValueSun); err != nil {
		return "", err
	}

	account, err := grpc.GetAccount(ownerBase58)
	if err != nil {
		return "", fmt.Errorf("tron get account: %w", err)
	}
	senderBalanceSun := big.NewInt(account.GetBalance())

	estimatedFeeSun := big.NewInt(constants.TronDefaultFeeLimitSun)
	if fee, err := tron.EstimateTronFeeSunWithPadding(ctx, grpc, ownerBase58, hinkalBase58, callData, callValueSun); err == nil {
		estimatedFeeSun = fee
	}
	totalRequiredSun := new(big.Int).Add(callValueSun, estimatedFeeSun)
	if senderBalanceSun.Cmp(totalRequiredSun) < 0 {
		return "", tron.ErrInsufficientTronBalance
	}

	sendTx, err := grpc.TriggerContractWithDataCtx(
		ctx,
		ownerBase58,
		hinkalBase58,
		callData,
		constants.TronDefaultFeeLimitSun,
		callValueSun.Int64(),
		"",
		0,
	)
	if err != nil {
		return "", fmt.Errorf("tron build prooflessDeposit: %w", err)
	}

	return client.SignAndBroadcast(ctx, sendTx.GetTransaction())
}

func hinkalTronProoflessDepositWithPublicFee(
	ctx context.Context,
	client tron.TronClient,
	chainID int,
	wrapperAddress string,
	feeRecipient string,
	feeToken string,
	feeAmount *big.Int,
	erc20Tokens []types.ERC20Token,
	amounts []*big.Int,
	stealthAddressStructures []types.StealthAddressStructure,
) (string, error) {
	grpc := client.GrpcClient()
	ownerBase58 := client.GetAddress()
	ownerHex, err := utils.AddressToHexFormat(ownerBase58)
	if err != nil {
		return "", err
	}

	wrapperHex, err := utils.AddressToHexFormat(wrapperAddress)
	if err != nil {
		return "", err
	}
	wrapperBase58, err := utils.EVMHexToTronBase58Address(wrapperHex)
	if err != nil {
		return "", err
	}
	feeRecipientHex, err := utils.AddressToHexFormat(feeRecipient)
	if err != nil {
		return "", err
	}
	feeTokenHex := constants.ZeroAddress
	if !strings.EqualFold(feeToken, constants.ZeroAddress) {
		feeTokenHex, err = utils.AddressToHexFormat(feeToken)
		if err != nil {
			return "", err
		}
	}

	erc20Addresses, err := tron.NormalizedTokenAddresses(erc20Tokens)
	if err != nil {
		return "", err
	}
	tokenIDs := make([]*big.Int, len(erc20Tokens))
	for i := range tokenIDs {
		tokenIDs[i] = big.NewInt(0)
	}

	callData, err := contractabi.PackProoflessDepositWithPublicFee(
		feeRecipientHex,
		feeTokenHex,
		feeAmount,
		erc20Addresses,
		amounts,
		tokenIDs,
		stealthAddressStructures,
	)
	if err != nil {
		return "", err
	}

	tokensForApproval, callValueSun := buildWrapperApprovalInputs(feeTokenHex, feeAmount, erc20Tokens, amounts)
	approvals, err := tronApprovalsFromTokenBalances(tokensForApproval)
	if err != nil {
		return "", err
	}
	if err := tron.ApproveTokens(ctx, client, chainID, ownerBase58, ownerHex, wrapperBase58, approvals); err != nil {
		return "", err
	}

	if err := tron.SimulateTronTransaction(ctx, chainID, wrapperBase58, ownerBase58, callData, callValueSun); err != nil {
		return "", err
	}

	account, err := grpc.GetAccount(ownerBase58)
	if err != nil {
		return "", fmt.Errorf("tron get account: %w", err)
	}
	senderBalanceSun := big.NewInt(account.GetBalance())

	estimatedFeeSun := big.NewInt(constants.TronDefaultFeeLimitSun)
	if fee, err := tron.EstimateTronFeeSunWithPadding(ctx, grpc, ownerBase58, wrapperBase58, callData, callValueSun); err == nil {
		estimatedFeeSun = fee
	}
	totalRequiredSun := new(big.Int).Add(callValueSun, estimatedFeeSun)
	if senderBalanceSun.Cmp(totalRequiredSun) < 0 {
		return "", tron.ErrInsufficientTronBalance
	}

	sendTx, err := grpc.TriggerContractWithDataCtx(
		ctx,
		ownerBase58,
		wrapperBase58,
		callData,
		constants.TronDefaultFeeLimitSun,
		callValueSun.Int64(),
		"",
		0,
	)
	if err != nil {
		return "", fmt.Errorf("tron build prooflessDeposit wrapper: %w", err)
	}

	return client.SignAndBroadcast(ctx, sendTx.GetTransaction())
}

func HinkalProoflessDepositWithPublicFee(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Token types.ERC20Token,
	amounts []*big.Int,
	stealthAddressStructures []types.StealthAddressStructure,
	feeAmount *big.Int,
) (string, error) {
	erc20Tokens := make([]types.ERC20Token, len(amounts))
	for i := range erc20Tokens {
		erc20Tokens[i] = erc20Token
	}

	if constants.IsSolanaLike(chainID) {
		return HinkalSolanaProoflessDepositWithPublicFee(ctx, hinkal, erc20Token, amounts, stealthAddressStructures, feeAmount)
	}

	if err := assertWrapperArgLengths("HinkalProoflessDepositWithPublicFee", erc20Tokens, amounts, stealthAddressStructures); err != nil {
		return "", err
	}
	if err := assertNoDuplicateStealthAddressStructures(stealthAddressStructures); err != nil {
		return "", err
	}

	contractData, err := constants.GetContractData(chainID)
	if err != nil {
		return "", err
	}
	if contractData.HinkalWrapperAddress == "" {
		if constants.IsTronLike(chainID) {
			return "", fmt.Errorf("transactions: no HinkalWrapper configured for chain %d", chainID)
		}
		_, txHash, err := HinkalProoflessDeposit(ctx, hinkal, erc20Tokens, amounts, stealthAddressStructures, false)
		return txHash, err
	}

	feeToken := erc20Token.Erc20TokenAddress
	feeRecipient, err := hinkal.GetRandomRelay(ctx, chainID, false)
	if err != nil {
		return "", err
	}
	if feeRecipient == "" {
		return "", fmt.Errorf("transactions: no relay available for chain %d", chainID)
	}

	if constants.IsTronLike(chainID) {
		client, err := hinkal.GetTronWeb()
		if err != nil {
			return "", err
		}
		return hinkalTronProoflessDepositWithPublicFee(
			ctx,
			client,
			chainID,
			contractData.HinkalWrapperAddress,
			feeRecipient,
			feeToken,
			feeAmount,
			erc20Tokens,
			amounts,
			stealthAddressStructures,
		)
	}

	erc20Addresses := make([]string, len(erc20Tokens))
	tokenIDs := make([]*big.Int, len(erc20Tokens))
	for i, token := range erc20Tokens {
		erc20Addresses[i] = token.Erc20TokenAddress
		tokenIDs[i] = big.NewInt(0)
	}

	tokensForApproval, ethValue := buildWrapperApprovalInputs(feeToken, feeAmount, erc20Tokens, amounts)
	data, err := contractabi.PackProoflessDepositWithPublicFee(
		feeRecipient,
		feeToken,
		feeAmount,
		erc20Addresses,
		amounts,
		tokenIDs,
		stealthAddressStructures,
	)
	if err != nil {
		return "", err
	}

	txReq := types.TransactionRequest{To: contractData.HinkalWrapperAddress, Data: data}
	if ethValue.Sign() > 0 {
		txReq.Value = ethValue
	}

	adapter, err := hinkal.GetProviderAdapter(&chainID)
	if err != nil {
		return "", err
	}

	approvalTokens := make([]types.ERC20Token, 0, len(tokensForApproval))
	approvalAmounts := make([]*big.Int, 0, len(tokensForApproval))
	requirements := make([]web3.ApprovalRequirement, 0, len(tokensForApproval))
	for _, twb := range tokensForApproval {
		if strings.EqualFold(twb.token.Erc20TokenAddress, constants.ZeroAddress) || twb.balance.Sign() <= 0 {
			continue
		}
		approvalTokens = append(approvalTokens, twb.token)
		approvalAmounts = append(approvalAmounts, twb.balance)
		requirements = append(requirements, web3.ApprovalRequirement{TokenAddress: twb.token.Erc20TokenAddress, RequiredAmount: twb.balance})
	}
	if len(approvalTokens) > 0 {
		wrapperAddr := common.HexToAddress(contractData.HinkalWrapperAddress)
		if err := web3.ApproveTokensToHinkal(ctx, adapter, chainID, wrapperAddr, approvalTokens, approvalAmounts); err != nil {
			return "", err
		}
		ownerAddr, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
		if err != nil {
			return "", err
		}
		if err := web3.WaitForErc20Approvals(ctx, adapter, chainID, common.HexToAddress(ownerAddr), wrapperAddr, requirements, 30, time.Second); err != nil {
			return "", err
		}
	}

	return adapter.SendTransaction(ctx, txReq)
}

func HinkalProoflessDeposit(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	erc20Tokens []types.ERC20Token,
	amountChanges []*big.Int,
	stealthAddressStructuresOverride []types.StealthAddressStructure,
	returnTxData bool,
) (types.TransactionRequest, string, error) {
	chainID, err := pretransaction.ValidateAndGetChainID(erc20Tokens)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if constants.IsSolanaLike(chainID) {
		return types.TransactionRequest{}, "", errProoflessNotImplemented
	}

	if len(erc20Tokens) != len(amountChanges) {
		return types.TransactionRequest{}, "", errTokenAmountLengthMismatch
	}

	stealthAddressStructures, err := getProoflessStealthAddressStructures(hinkal, len(erc20Tokens), stealthAddressStructuresOverride)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if err := assertNoDuplicateStealthAddressStructures(stealthAddressStructures); err != nil {
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
		txid, err := hinkalTronProoflessDeposit(ctx, client, chainID, erc20Tokens, amountChanges, stealthAddressStructures)
		return types.TransactionRequest{}, txid, err
	}

	hinkalAddr, err := constants.HinkalAddress(chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	erc20Addresses := make([]string, len(erc20Tokens))
	tokenIDs := make([]*big.Int, len(erc20Tokens))
	for i, token := range erc20Tokens {
		erc20Addresses[i] = token.Erc20TokenAddress
		tokenIDs[i] = big.NewInt(0)
	}

	data, err := contractabi.PackProoflessDeposit(chainID, erc20Addresses, amountChanges, tokenIDs, stealthAddressStructures)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	ethAmount := big.NewInt(0)
	for i, token := range erc20Tokens {
		if strings.EqualFold(token.Erc20TokenAddress, constants.ZeroAddress) {
			ethAmount.Add(ethAmount, amountChanges[i])
		}
	}
	var value *big.Int
	if ethAmount.Sign() > 0 {
		value = ethAmount
	}

	txReq := types.TransactionRequest{To: hinkalAddr, Data: data, Value: value}
	if returnTxData {
		return txReq, "", nil
	}

	adapter, err := hinkal.GetProviderAdapter(&chainID)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}

	tokensWithBalances := aggregateAmountsForApproval(erc20Tokens, amountChanges)
	approvalTokens := make([]types.ERC20Token, 0, len(tokensWithBalances))
	approvalAmounts := make([]*big.Int, 0, len(tokensWithBalances))
	requirements := make([]web3.ApprovalRequirement, 0, len(tokensWithBalances))
	for _, twb := range tokensWithBalances {
		if strings.EqualFold(twb.token.Erc20TokenAddress, constants.ZeroAddress) || twb.balance.Sign() <= 0 {
			continue
		}
		approvalTokens = append(approvalTokens, twb.token)
		approvalAmounts = append(approvalAmounts, twb.balance)
		requirements = append(requirements, web3.ApprovalRequirement{TokenAddress: twb.token.Erc20TokenAddress, RequiredAmount: twb.balance})
	}

	if len(approvalTokens) > 0 {
		if err := web3.ApproveTokensToHinkal(ctx, adapter, chainID, common.HexToAddress(hinkalAddr), approvalTokens, approvalAmounts); err != nil {
			return types.TransactionRequest{}, "", err
		}
		ownerAddr, err := hinkal.GetEthereumAddressByChain(ctx, chainID)
		if err != nil {
			return types.TransactionRequest{}, "", err
		}
		if err := web3.WaitForErc20Approvals(ctx, adapter, chainID, common.HexToAddress(ownerAddr), common.HexToAddress(hinkalAddr), requirements, 30, time.Second); err != nil {
			return types.TransactionRequest{}, "", err
		}
	}

	txHash, err := adapter.SendTransaction(ctx, txReq)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	return txReq, txHash, nil
}
