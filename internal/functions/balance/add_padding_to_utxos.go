package balance

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	errorhandling "github.com/gioeba/go_sdk_test/error-handling"
	solanautils "github.com/gioeba/go_sdk_test/internal/functions/solana"
	"github.com/gioeba/go_sdk_test/internal/functions/web3"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

func AddPaddingToUtxos(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	chainID int,
	erc20Addresses []string,
	amountChanges []*big.Int,
	maxInput int,
	tokenIDs []int,
	forceEmptyUtxos bool,
	useBlockedUtxos bool,
) ([][]*utxo.Utxo, error) {
	if maxInput == 0 {
		maxInput = 6
	}
	if tokenIDs == nil {
		tokenIDs = make([]int, len(erc20Addresses))
	}

	userKeys := hinkal.GetUserKeys()
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}
	spendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}
	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}

	if len(erc20Addresses) == 0 {
		return [][]*utxo.Utxo{}, nil
	}

	ethAddress := ""
	if useBlockedUtxos {
		ethAddress, err = hinkal.GetEthereumAddressByChain(ctx, chainID)
		if err != nil {
			return nil, err
		}
	}

	ensuredTokensWithID := make([]types.TokenWithID, len(erc20Addresses))
	for i := range erc20Addresses {
		ensuredTokensWithID[i] = types.TokenWithID{Erc20TokenAddress: erc20Addresses[i], TokenID: tokenIDs[i]}
	}

	inputUtxosPerToken, err := GetInputUtxoAndBalancePerToken(ctx, InputUtxoParams{
		Hinkal:                hinkal,
		ChainID:               chainID,
		EthAddress:            ethAddress,
		AllowRemoteDecryption: hinkal.GenerateProofRemotely(),
		UseBlockedUtxos:       useBlockedUtxos,
	}, 2, false, nil, ensuredTokensWithID)
	if err != nil {
		return nil, err
	}

	inputUtxosArrayToBePadded := make([][]*utxo.Utxo, 0, len(erc20Addresses))
	maxUtxoNum := 0
	for i := 0; i < len(erc20Addresses); i++ {
		if !forceEmptyUtxos {
			key := encodeTokenWithID(chainID, types.TokenWithID{Erc20TokenAddress: erc20Addresses[i], TokenID: tokenIDs[i]})
			inputUtxos := inputUtxosPerToken[key]
			if len(inputUtxos) > maxUtxoNum {
				maxUtxoNum = len(inputUtxos)
			}
			inputUtxosArrayToBePadded = append(inputUtxosArrayToBePadded, inputUtxos)
		} else {
			inputUtxosArrayToBePadded = append(inputUtxosArrayToBePadded, []*utxo.Utxo{})
		}
	}

	if maxUtxoNum == 2 {
		return inputUtxosArrayToBePadded, nil
	}

	inputUtxosArrayPadded := make([][]*utxo.Utxo, 0, len(inputUtxosArrayToBePadded))
	for i, utxos := range inputUtxosArrayToBePadded {
		if len(utxos) > maxInput {
			firstSixUtxos := utxos[:maxInput]
			firstSixAmount := new(big.Int)
			for _, u := range firstSixUtxos {
				firstSixAmount.Add(firstSixAmount, u.Amount)
			}
			if amountChanges[i].Sign() < 0 && firstSixAmount.Cmp(new(big.Int).Neg(amountChanges[i])) < 0 {
				if err := overLimitError(firstSixAmount, erc20Addresses[i], chainID); err != nil {
					return nil, err
				}
			}
			inputUtxosArrayPadded = append(inputUtxosArrayPadded, firstSixUtxos)
		} else {
			tempUtxosStorage := append([]*utxo.Utxo{}, utxos...)
			diff := maxInput - len(utxos)
			for diff > 0 {
				diff--
				mintAddress := ""
				modifiedErc20TokenAddress := erc20Addresses[i]
				if constants.IsSolanaLike(chainID) {
					mintAddress = erc20Addresses[i]
					formatted, err := solanautils.FormatMintAddress(erc20Addresses[i])
					if err != nil {
						return nil, err
					}
					modifiedErc20TokenAddress = formatted.CompressedAddress
				}
				padUtxo, err := utxo.NewUtxo(types.UtxoParams{
					Amount:            big.NewInt(0),
					Erc20TokenAddress: modifiedErc20TokenAddress,
					MintAddress:       mintAddress,
					NullifyingKey:     shieldedPrivateKey,
					SpendingPublicKey: spendingPublicKey,
					IsNewStyle:        true,
				})
				if err != nil {
					return nil, err
				}
				tempUtxosStorage = append(tempUtxosStorage, padUtxo)
			}
			inputUtxosArrayPadded = append(inputUtxosArrayPadded, tempUtxosStorage)
		}
	}

	return inputUtxosArrayPadded, nil
}

func overLimitError(firstSixAmount *big.Int, erc20Address string, chainID int) error {
	token := constants.GetERC20Token(erc20Address, chainID)
	if token == nil {
		return &errorhandling.ErrorWithAmount{Amount: 0, Message: errorhandling.ErrCodeUtxoLimitations}
	}
	hintPrecision := 2
	if token.Decimals == 18 {
		hintPrecision = 6
	}
	amountStr, err := web3.GetAmountWithPrecision(firstSixAmount, *token, hintPrecision)
	if err != nil {
		return err
	}
	amountInToken, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return err
	}
	message := fmt.Sprintf("%s. Please try again with %s %s, including gas fees.",
		errorhandling.ErrCodeUtxoLimitations, strconv.FormatFloat(amountInToken, 'g', -1, 64), token.Symbol)
	return &errorhandling.ErrorWithAmount{Amount: amountInToken, Message: message}
}
