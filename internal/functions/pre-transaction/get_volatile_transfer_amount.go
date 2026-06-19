package pretransaction

import (
	"context"
	"log"
	"math/big"

	"github.com/gioeba/go_sdk_test/internal/api"
	tokenchecker "github.com/gioeba/go_sdk_test/internal/data-structures/token-checker"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/types"
)

func ModifyVolatileTokenAmountChanges(
	ctx context.Context,
	chainID int,
	tokens []types.ERC20Token,
	amountChanges []*big.Int,
	targetAddress string,
) []*big.Int {
	var volatileTokenSpends []types.VolatileTokenChange
	for index, token := range tokens {
		if !tokenchecker.IsPotentiallyVolatile(token) {
			continue
		}
		change := types.VolatileTokenChange{
			TokenAddress:  token.Erc20TokenAddress,
			Amount:        amountChanges[index].String(),
			OriginalIndex: index,
		}
		if targetAddress != "" {
			change.TargetAddress = targetAddress
		}
		volatileTokenSpends = append(volatileTokenSpends, change)
	}

	if len(volatileTokenSpends) == 0 {
		return amountChanges
	}

	results, err := api.SimulateVolatileTokenTransfer(ctx, chainID, volatileTokenSpends)
	if err != nil {
		log.Printf("Volatile transfer simulation failed: %v", err)
		return amountChanges
	}

	newAmountChanges := make([]*big.Int, len(amountChanges))
	for index, amount := range amountChanges {
		indexOfResult := -1
		for j, spend := range volatileTokenSpends {
			if spend.OriginalIndex == index {
				indexOfResult = j
				break
			}
		}
		if indexOfResult != -1 && indexOfResult < len(results) && results[indexOfResult].Success {
			if newAmount := results[indexOfResult].BalanceDifference; newAmount != "" {
				if parsed, parseErr := utils.ParseBigInt(newAmount); parseErr == nil && parsed.Sign() != 0 {
					newAmountChanges[index] = parsed
					continue
				}
			}
		}
		newAmountChanges[index] = amount
	}

	return newAmountChanges
}
