package hinkal

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	pretransaction "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
	"github.com/gioeba/go_sdk_test/internal/functions/transactions"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

var errInvalidSwapperAccountSalt = errors.New("hinkal: invalid swapperAccountSalt in swap data")

func resolveERC20Tokens(ctx context.Context, chainID int, erc20Addresses []string) ([]types.ERC20Token, error) {
	return web3.ResolveERC20TokensStrict(ctx, chainID, erc20Addresses)
}

func resolveERC20Token(ctx context.Context, chainID int, erc20Address string) (types.ERC20Token, error) {
	return web3.ResolveERC20TokenStrict(ctx, chainID, erc20Address)
}

func (h *Hinkal) GetRecipientInfo() (string, error) {
	return pretransaction.GetRecipientInfoFromUserKeys(h.UserKeys)
}

func (h *Hinkal) Deposit(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	preEstimateGas bool,
	returnTxData bool,
) (types.TransactionRequest, string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	return transactions.HinkalDeposit(ctx, h, erc20Tokens, amountChanges, preEstimateGas, returnTxData)
}

func (h *Hinkal) DepositForOther(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	recipientInfo string,
	preEstimateGas bool,
	returnTxData bool,
) (types.TransactionRequest, string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	return transactions.HinkalDepositForOther(ctx, h, erc20Tokens, amountChanges, recipientInfo, preEstimateGas, returnTxData)
}

func (h *Hinkal) DepositSolana(
	ctx context.Context,
	chainID int,
	erc20Address string,
	amount *big.Int,
	returnTxData bool,
) (string, error) {
	token, err := resolveERC20Token(ctx, chainID, erc20Address)
	if err != nil {
		return "", err
	}
	return transactions.HinkalSolanaDeposit(ctx, h, amount, token, returnTxData)
}

func (h *Hinkal) DepositSolanaForOther(
	ctx context.Context,
	chainID int,
	erc20Address string,
	amount *big.Int,
	recipientInfo string,
	returnTxData bool,
) (string, error) {
	token, err := resolveERC20Token(ctx, chainID, erc20Address)
	if err != nil {
		return "", err
	}
	return transactions.HinkalSolanaDepositForOther(ctx, h, amount, token, recipientInfo, returnTxData)
}

func (h *Hinkal) ProoflessDeposit(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	stealthAddressStructures []types.StealthAddressStructure,
	returnTxData bool,
) (types.TransactionRequest, string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if constants.IsSolanaLike(chainID) {
		txDataOrSignature, err := transactions.HinkalSolanaProoflessDeposit(ctx, h, erc20Tokens, amountChanges, stealthAddressStructures, returnTxData)
		return types.TransactionRequest{}, txDataOrSignature, err
	}
	return transactions.HinkalProoflessDeposit(ctx, h, erc20Tokens, amountChanges, stealthAddressStructures, returnTxData)
}

func (h *Hinkal) ProoflessDepositWithPublicFee(
	ctx context.Context,
	chainID int,
	erc20Address string,
	amounts []*big.Int,
	stealthAddressStructures []types.StealthAddressStructure,
	feeAmount *big.Int,
) (string, error) {
	erc20Token, err := resolveERC20Token(ctx, chainID, erc20Address)
	if err != nil {
		return "", err
	}
	return transactions.HinkalProoflessDepositWithPublicFee(ctx, h, chainID, erc20Token, amounts, stealthAddressStructures, feeAmount)
}

func (h *Hinkal) Withdraw(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	recipientAddress string,
	isRelayerOff bool,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (types.TransactionRequest, string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return types.TransactionRequest{}, "", err
	}
	if constants.IsSolanaLike(chainID) {
		txHash, err := transactions.HinkalSolanaWithdraw(ctx, h, erc20Tokens, amountChanges, recipientAddress, feeToken, feeStructureOverride)
		return types.TransactionRequest{}, txHash, err
	}
	return transactions.HinkalWithdraw(
		ctx,
		h,
		erc20Tokens,
		amountChanges,
		recipientAddress,
		isRelayerOff,
		feeToken,
		feeStructureOverride,
	)
}

func (h *Hinkal) Transfer(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	recipientAddress string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return "", err
	}
	if constants.IsSolanaLike(chainID) {
		return transactions.HinkalSolanaTransfer(ctx, h, erc20Tokens, amountChanges, recipientAddress, feeToken, feeStructureOverride)
	}
	return transactions.HinkalTransfer(ctx, h, erc20Tokens, amountChanges, recipientAddress, feeToken, feeStructureOverride)
}

func (h *Hinkal) ClaimUtxo(
	ctx context.Context,
	chainID int,
	erc20Address string,
	claimableUtxo *utxo.Utxo,
	feeStructureOverride *types.FeeStructure,
	claimableSignature string,
) (string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, []string{erc20Address})
	if err != nil {
		return "", err
	}
	if constants.IsSolanaLike(chainID) {
		return transactions.HinkalSolanaClaimUtxo(ctx, h, erc20Tokens, claimableUtxo, feeStructureOverride, claimableSignature)
	}
	return transactions.HinkalClaimUtxo(ctx, h, erc20Tokens, claimableUtxo, feeStructureOverride, claimableSignature)
}

func (h *Hinkal) DepositAndWithdraw(
	ctx context.Context,
	chainID int,
	erc20Address string,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
	preEstimateGas bool,
) (types.DepositAndSendExtendedResult, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, []string{erc20Address})
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	if constants.IsSolanaLike(chainID) {
		return transactions.HinkalSolanaDepositAndWithdraw(
			ctx,
			h,
			erc20Tokens,
			recipientAmounts,
			recipientAddresses,
			txCompletionTime,
			feeStructureOverride,
		)
	}
	return transactions.HinkalDepositAndWithdraw(
		ctx,
		h,
		erc20Tokens,
		recipientAmounts,
		recipientAddresses,
		txCompletionTime,
		feeStructureOverride,
		preEstimateGas,
	)
}

func (h *Hinkal) DepositAndBridge(
	ctx context.Context,
	chainID int,
	erc20Address string,
	recipients []types.BridgeRecipient,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
	preEstimateGas bool,
) (types.DepositAndSendExtendedResult, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, []string{erc20Address})
	if err != nil {
		return types.DepositAndSendExtendedResult{}, err
	}
	return transactions.HinkalDepositAndBridge(
		ctx,
		h,
		erc20Tokens,
		recipients,
		txCompletionTime,
		feeStructureOverride,
		preEstimateGas,
	)
}

func (h *Hinkal) NearDepositAndBridge(
	ctx context.Context,
	chainID int,
	erc20Address string,
	recipientAmounts []*big.Int,
	recipientAddresses []string,
	params types.NearBridgeParams,
	txCompletionTime *int,
	feeStructureOverride *types.FeeStructure,
) (types.NearBridgeResult, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, []string{erc20Address})
	if err != nil {
		return types.NearBridgeResult{}, err
	}
	return transactions.HinkalNearDepositAndBridge(
		ctx,
		h,
		erc20Tokens,
		recipientAmounts,
		recipientAddresses,
		params,
		txCompletionTime,
		feeStructureOverride,
	)
}

func (h *Hinkal) WithdrawStuckUtxos(
	ctx context.Context,
	chainID int,
	erc20Address string,
	recipientAddress string,
) ([]string, error) {
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, []string{erc20Address})
	if err != nil {
		return nil, err
	}
	return transactions.HinkalWithdrawStuckUtxos(ctx, h, erc20Tokens, recipientAddress)
}

func (h *Hinkal) CheckSendTransactionStatus(ctx context.Context, scheduleID string) (types.ScheduledTransactionByIDResponse, error) {
	return api.GetScheduledTransactionByID(ctx, scheduleID)
}

func (h *Hinkal) Swap(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	deltaAmounts []*big.Int,
	externalActionID types.ExternalActionID,
	swapData string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
	if constants.IsSolanaLike(chainID) {
		return h.SwapSolana(ctx, chainID, erc20Addresses, deltaAmounts, swapData, feeToken, feeStructureOverride)
	}
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return "", err
	}
	return transactions.HinkalSwap(ctx, h, erc20Tokens, deltaAmounts, externalActionID, swapData, feeToken, feeStructureOverride)
}

func (h *Hinkal) SwapSolana(
	ctx context.Context,
	chainID int,
	erc20Addresses []string,
	deltaAmounts []*big.Int,
	swapData string,
	feeToken string,
	feeStructureOverride *types.FeeStructure,
) (string, error) {
	var parsed struct {
		api.OKXSwapResponse
		SwapperAccountSalt string `json:"swapperAccountSalt"`
	}
	if err := json.Unmarshal([]byte(swapData), &parsed); err != nil {
		return "", err
	}
	salt, ok := new(big.Int).SetString(parsed.SwapperAccountSalt, 10)
	if !ok {
		return "", errInvalidSwapperAccountSalt
	}
	erc20Tokens, err := resolveERC20Tokens(ctx, chainID, erc20Addresses)
	if err != nil {
		return "", err
	}
	return transactions.HinkalSolanaSwap(
		ctx,
		h,
		erc20Tokens,
		deltaAmounts,
		salt,
		parsed.Data.InstructionLists,
		parsed.Data.AddressLookupTableAccount,
		feeToken,
		feeStructureOverride,
	)
}
