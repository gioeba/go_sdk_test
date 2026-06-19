// Package fees exposes the Hinkal SDK fee calculations.
package fees

import (
	impl "github.com/gioeba/go_sdk_test/internal/functions/fees"
	pretx "github.com/gioeba/go_sdk_test/internal/functions/pre-transaction"
)

var GetFeeStructure = pretx.GetFeeStructure

var (
	CalculateTotalFee             = impl.CalculateTotalFee
	CalculateWithdrawalAmount     = impl.CalculateWithdrawalAmount
	CalculateModifiedFeeStructure = impl.CalculateModifiedFeeStructure
	GetGasTokenSymbols            = impl.GetGasTokenSymbols
)
