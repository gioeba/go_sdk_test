package errorhandling

import (
	"math/big"

	"github.com/gioeba/go_sdk_test/types"
)

const (
	ErrCodeInsufficientFundsToTransact = "not enough funds to transact"
	ErrCodeDecimalsLimit               = "number of decimals exceed maximum"
	ErrCodeUtxoLimitations             = "Your transaction exceeds the UTXO limit"
	ErrCodeFeesOverTransactionValue    = "Fees are over transaction value. Please, increase the amount"
)

type ErrorWithAmount struct {
	Amount  float64
	Message string
}

func (e *ErrorWithAmount) Error() string { return e.Message }

type FeeOverTransactionValueError struct {
	TotalFeeWEI *big.Int
	FeeUnit     *types.ERC20Token
}

func (e *FeeOverTransactionValueError) Error() string { return ErrCodeFeesOverTransactionValue }

type ErrorWithRelayerTransaction struct {
	Message string
	TxHash  string
}

func (e *ErrorWithRelayerTransaction) Error() string { return e.Message }
