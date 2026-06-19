package balance

import (
	"context"
	"math/big"
	"sort"
	"strings"

	"github.com/gioeba/go_sdk_test/constants"
	solanautils "github.com/gioeba/go_sdk_test/internal/functions/solana"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

func encodeTokenWithID(chainID int, tokenWithID types.TokenWithID) string {
	key := tokenWithID.Erc20TokenAddress + "-0"
	if constants.IsSolanaLike(chainID) {
		return key
	}
	return strings.ToLower(key)
}

func GetInputUtxoAndBalancePerToken(
	ctx context.Context,
	p InputUtxoParams,
	minInput int,
	sliceIfMore6 bool,
	tokensWithID []types.TokenWithID,
	ensuredTokensWithID []types.TokenWithID,
) (map[string][]*utxo.Utxo, error) {
	userKeys := p.Hinkal.GetUserKeys()
	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}
	spendingPublicKey := []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]}

	var inputUtxos []*utxo.Utxo
	if p.UseBlockedUtxos {
		inputUtxos, err = GetInputUtxoAndBalanceOfStuckUtxos(ctx, p)
	} else {
		inputUtxos, err = GetInputUtxoAndBalance(ctx, p)
	}
	if err != nil {
		return nil, err
	}

	inputUtxosPerToken := map[string][]*utxo.Utxo{}
	var keyOrder []string
	addKey := func(key string) {
		if _, ok := inputUtxosPerToken[key]; !ok {
			inputUtxosPerToken[key] = nil
			keyOrder = append(keyOrder, key)
		}
	}

	for _, item := range inputUtxos {
		tokenAddress, err := item.GetTokenAddress(p.ChainID)
		if err != nil {
			continue
		}
		if tokensWithID != nil && !anyTokenAddressMatches(tokensWithID, tokenAddress) {
			continue
		}
		key := encodeTokenWithID(p.ChainID, types.TokenWithID{Erc20TokenAddress: tokenAddress})
		addKey(key)
		inputUtxosPerToken[key] = append(inputUtxosPerToken[key], item)
	}

	for _, item := range ensuredTokensWithID {
		addKey(encodeTokenWithID(p.ChainID, types.TokenWithID{Erc20TokenAddress: item.Erc20TokenAddress}))
	}

	for _, key := range keyOrder {
		erc20TokenAddress := strings.Split(key, "-")[0]
		mintAddress := ""
		modifiedErc20TokenAddress := strings.ToLower(erc20TokenAddress)
		if constants.IsSolanaLike(p.ChainID) {
			mintAddress = erc20TokenAddress
			formatted, err := solanautils.FormatMintAddress(erc20TokenAddress)
			if err != nil {
				return nil, err
			}
			modifiedErc20TokenAddress = strings.ToLower(formatted.CompressedAddress)
		}
		padded, err := sortAndPadUtxos(inputUtxosPerToken[key], minInput, sliceIfMore6, shieldedPrivateKey, spendingPublicKey, modifiedErc20TokenAddress, mintAddress)
		if err != nil {
			return nil, err
		}
		inputUtxosPerToken[key] = padded
	}

	return inputUtxosPerToken, nil
}

func sortAndPadUtxos(
	inputUtxos []*utxo.Utxo,
	minInput int,
	sliceIfMore6 bool,
	shieldedPrivateKey string,
	spendingPublicKey []*big.Int,
	erc20TokenAddress string,
	mintAddress string,
) ([]*utxo.Utxo, error) {
	sort.SliceStable(inputUtxos, func(i, j int) bool {
		return inputUtxos[i].Amount.Cmp(inputUtxos[j].Amount) > 0
	})

	for len(inputUtxos) < minInput || (len(inputUtxos) > minInput && len(inputUtxos) < 6) {
		padUtxo, err := utxo.NewUtxo(types.UtxoParams{
			Amount:            big.NewInt(0),
			Erc20TokenAddress: erc20TokenAddress,
			MintAddress:       mintAddress,
			NullifyingKey:     shieldedPrivateKey,
			SpendingPublicKey: spendingPublicKey,
			IsNewStyle:        true,
		})
		if err != nil {
			return nil, err
		}
		inputUtxos = append(inputUtxos, padUtxo)

		if sliceIfMore6 {
			for len(inputUtxos) > 6 {
				inputUtxos = inputUtxos[:len(inputUtxos)-1]
			}
		}
	}

	return inputUtxos, nil
}

func anyTokenAddressMatches(tokensWithID []types.TokenWithID, tokenAddress string) bool {
	for _, t := range tokensWithID {
		if strings.EqualFold(t.Erc20TokenAddress, tokenAddress) {
			return true
		}
	}
	return false
}
