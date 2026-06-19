package pretransaction

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/gioeba/go_sdk_test/cryptokeys"
	errorhandling "github.com/gioeba/go_sdk_test/error-handling"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

func countTotalAmountInUtxos(utxos []*utxo.Utxo) *big.Int {
	total := new(big.Int)
	for _, u := range utxos {
		total.Add(total, u.Amount)
	}
	return total
}

func OutputUtxoProcessing(
	userKeys *cryptokeys.UserKeys,
	inputUtxos []*utxo.Utxo,
	amountChange *big.Int,
	timeStamp string,
	revertIfNegative bool,
	recipientAddress string,
	recipientAmountChange *big.Int,
) ([]*utxo.Utxo, error) {
	totalAmount := countTotalAmountInUtxos(inputUtxos)
	erc20TokenAddress := inputUtxos[0].Erc20TokenAddress
	mintAddress := inputUtxos[0].MintAddress

	if revertIfNegative && amountChange.Sign() < 0 && new(big.Int).Add(totalAmount, amountChange).Sign() < 0 {
		return nil, errors.New(errorhandling.ErrCodeInsufficientFundsToTransact)
	}

	shieldedPrivateKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return nil, err
	}

	changeUtxo, err := utxo.NewUtxo(types.UtxoParams{
		Amount:            utils.BigintMax(new(big.Int).Add(totalAmount, amountChange), big.NewInt(0)),
		Erc20TokenAddress: erc20TokenAddress,
		MintAddress:       mintAddress,
		NullifyingKey:     shieldedPrivateKey,
		TimeStamp:         timeStamp,
		SpendingPublicKey: []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]},
		IsNewStyle:        true,
	})
	if err != nil {
		return nil, err
	}
	outputUtxos := []*utxo.Utxo{changeUtxo}

	if recipientAddress != "" && recipientAmountChange != nil {
		parts := strings.Split(recipientAddress, ",")
		if len(parts) < 5 {
			return nil, fmt.Errorf("outputUtxoProcessing: malformed recipient address %q", recipientAddress)
		}
		stealthAddress, h00, h01, encryptionKey := parts[0], parts[1], parts[2], parts[4]
		h00Big, err := utils.ParseBigInt(h00)
		if err != nil {
			return nil, err
		}
		h01Big, err := utils.ParseBigInt(h01)
		if err != nil {
			return nil, err
		}
		recipientUtxo, err := utxo.NewUtxo(types.UtxoParams{
			Amount:            recipientAmountChange,
			Erc20TokenAddress: erc20TokenAddress,
			MintAddress:       mintAddress,
			H0:                &types.JubPoint{h00Big, h01Big},
			StealthAddress:    stealthAddress,
			EncryptionKey:     encryptionKey,
			TimeStamp:         timeStamp,
			IsNewStyle:        true,
		})
		if err != nil {
			return nil, err
		}
		outputUtxos = append(outputUtxos, recipientUtxo)
	}

	return outputUtxos, nil
}
