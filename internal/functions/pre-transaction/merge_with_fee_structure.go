package pretransaction

import (
	"math/big"
	"strings"

	"github.com/gioeba/go_sdk_test/constants"
	errorhandling "github.com/gioeba/go_sdk_test/error-handling"
	"github.com/gioeba/go_sdk_test/types"
)

func MergeWithFeeStructure(
	chainID int,
	erc20Addresses *[]string,
	amountChanges *[]*big.Int,
	feeStructure types.FeeStructure,
) error {
	feeTokenIndex := -1
	for i, tokenAddress := range *erc20Addresses {
		if strings.EqualFold(tokenAddress, feeStructure.FeeToken) {
			feeTokenIndex = i
			break
		}
	}

	if feeTokenIndex == -1 {
		*erc20Addresses = append(*erc20Addresses, feeStructure.FeeToken)
		*amountChanges = append(*amountChanges, new(big.Int).Neg(feeStructure.FlatFee))
		return nil
	}

	changes := *amountChanges
	if changes[feeTokenIndex].Sign() > 0 && changes[feeTokenIndex].Cmp(feeStructure.FlatFee) < 0 {
		return &errorhandling.FeeOverTransactionValueError{
			TotalFeeWEI: feeStructure.FlatFee,
			FeeUnit:     constants.GetERC20Token(feeStructure.FeeToken, chainID),
		}
	}

	changes[feeTokenIndex] = new(big.Int).Sub(changes[feeTokenIndex], feeStructure.FlatFee)
	return nil
}
