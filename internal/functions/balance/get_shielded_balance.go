package balance

import (
	"context"
	"math/big"
	"strings"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

func GetShieldedBalance(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	passedShieldedPublicKey string,
	ethAddress string,
	resetCacheBefore bool,
	allowRemoteDecryption bool,
	useBlockedUtxos bool,
) (map[string]types.TokenBalance, error) {
	mutex := utils.GetChainBalanceFetchingMutex(chainID)
	mutex.Lock()
	defer mutex.Unlock()

	params := InputUtxoParams{
		Hinkal:                  hinkal,
		ChainID:                 chainID,
		PassedShieldedPublicKey: passedShieldedPublicKey,
		EthAddress:              ethAddress,
		ResetCacheBefore:        resetCacheBefore,
		AllowRemoteDecryption:   allowRemoteDecryption,
	}

	var inputUtxos []*utxo.Utxo
	var err error
	if useBlockedUtxos {
		inputUtxos, err = GetInputUtxoAndBalanceOfStuckUtxos(ctx, params)
	} else {
		inputUtxos, err = GetInputUtxoAndBalance(ctx, params)
	}
	if err != nil {
		return nil, err
	}

	tokenRegistry := constants.GetERC20Registry(chainID)
	balancesMap := make(map[string]types.TokenBalance, len(tokenRegistry))
	for _, token := range tokenRegistry {
		balance := new(big.Int)
		timestamp := ""
		for _, u := range inputUtxos {
			tokenAddress, err := u.GetTokenAddress(chainID)
			if err != nil {
				continue
			}
			if !strings.EqualFold(token.Erc20TokenAddress, tokenAddress) {
				continue
			}
			balance.Add(balance, u.Amount)
			if timestamp == "" {
				timestamp = u.TimeStamp
			}
		}
		balancesMap[strings.ToLower(token.Erc20TokenAddress)] = types.TokenBalance{
			Token:     token,
			Balance:   balance,
			Timestamp: timestamp,
		}
	}

	return balancesMap, nil
}
