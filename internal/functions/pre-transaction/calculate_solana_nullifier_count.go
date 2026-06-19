package pretransaction

import (
	"context"
	"math/big"
	"strings"

	solana "github.com/gagliardetto/solana-go"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	errorhandling "github.com/gioeba/go_sdk_test/error-handling"
	"github.com/gioeba/go_sdk_test/internal/functions/balance"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

func CalculateSolanaNullifierCount(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
) int {
	if !constants.IsSolanaLike(chainID) || len(mintAddresses) == 0 || len(amountChanges) == 0 {
		return 0
	}
	count, err := solanaNullifierCount(ctx, hinkal, chainID, mintAddresses, amountChanges)
	if err != nil {
		if strings.Contains(err.Error(), errorhandling.ErrCodeUtxoLimitations) {
			return 6
		}
		return 0
	}
	return count
}

func solanaNullifierCount(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	mintAddresses []string,
	amountChanges []*big.Int,
) (int, error) {
	inputUtxosArray, err := balance.AddPaddingToUtxos(ctx, hinkal, chainID, mintAddresses, amountChanges, 6, nil, false, false)
	if err != nil {
		return 0, err
	}

	var nonZeroUtxos []*utxo.Utxo
	for _, group := range inputUtxosArray {
		for _, u := range group {
			if u.Amount != nil && u.Amount.Sign() != 0 {
				nonZeroUtxos = append(nonZeroUtxos, u)
			}
		}
	}
	if len(nonZeroUtxos) == 0 {
		return 0, nil
	}

	programID, err := solana.PublicKeyFromBase58(hinkal.HinkalAddress(chainID))
	if err != nil {
		return 0, err
	}
	originalDeployerStr, err := constants.OriginalDeployer(chainID)
	if err != nil {
		return 0, err
	}
	originalDeployer, err := solana.PublicKeyFromBase58(originalDeployerStr)
	if err != nil {
		return 0, err
	}

	pdasToCheck := make([]solana.PublicKey, len(nonZeroUtxos))
	for i, u := range nonZeroUtxos {
		nullifierStr, err := u.GetNullifier()
		if err != nil {
			return 0, err
		}
		nullifier, err := utils.ParseBigInt(nullifierStr)
		if err != nil {
			return 0, err
		}
		pda, err := GetNullifierAccount(nullifier, originalDeployer, programID)
		if err != nil {
			return 0, err
		}
		pdasToCheck[i] = pda
	}

	connection, err := hinkal.GetSolanaConnection()
	if err != nil {
		return 0, err
	}
	res, err := connection.GetMultipleAccounts(ctx, pdasToCheck...)
	if err != nil {
		return 0, err
	}

	missing := 0
	for _, account := range res.Value {
		if account == nil {
			missing++
		}
	}
	return missing, nil
}
