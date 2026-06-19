package pretransaction

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

var newStyleBit = new(big.Int).Lsh(big.NewInt(1), 255)

var rejectURLs = []string{"http://", "https://", "/payment/", ".app/", ".com/", ".netlify."}

// IsValidPrivateAddress mirrors @hinkal/common's isValidPrivateAddress: it validates the
// comma-separated recipient info (stealthAddress,H0[0],H0[1],H1[1],encryptionKey).
func IsValidPrivateAddress(address string) bool {
	for _, url := range rejectURLs {
		if strings.Contains(address, url) {
			return false
		}
	}
	parts := strings.Split(address, ",")
	if len(parts) < 5 {
		return false
	}
	stealthAddress, h00, h01, h11, encryptionKey := parts[0], parts[1], parts[2], parts[3], parts[4]
	if stealthAddress == "" || encryptionKey == "" || h00 == "" || h01 == "" || h11 == "" {
		return false
	}
	if !strings.HasPrefix(stealthAddress, "0x") || !strings.HasPrefix(encryptionKey, "0x") {
		return false
	}
	if len(encryptionKey) != 66 || len(stealthAddress) > 66 || len(stealthAddress) < 64 {
		return false
	}
	if strings.Contains(address, `"`) {
		return false
	}
	return true
}

// ConstructStealthAddressStructure parses a recipientInfo string
// (stealthAddress,H0[0],H0[1],H1[1],encryptionKey) into a StealthAddressStructure.
func ConstructStealthAddressStructure(recipientInfo string) (types.StealthAddressStructure, error) {
	parts := strings.Split(recipientInfo, ",")
	if len(parts) < 4 {
		return types.StealthAddressStructure{}, fmt.Errorf("constructStealthAddressStructure: malformed recipient info %q", recipientInfo)
	}
	stealthAddress, err := utils.ParseBigInt(parts[0])
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	h00, err := utils.ParseBigInt(parts[1])
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	h01, err := utils.ParseBigInt(parts[2])
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	h11, err := utils.ParseBigInt(parts[3])
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	return types.StealthAddressStructure{
		ExtraRandomization: new(big.Int).Or(h00, newStyleBit),
		StealthAddress:     stealthAddress,
		H0:                 h01,
		H1:                 h11,
	}, nil
}

func GetRecipientInfoFromUserKeys(userKeys *cryptokeys.UserKeys) (string, error) {
	nullifyingKey, err := userKeys.GetShieldedPrivateKey()
	if err != nil {
		return "", err
	}
	encPair, err := cryptokeys.GetEncryptionKeyPair(nullifyingKey)
	if err != nil {
		return "", err
	}
	spendingKeyPair, err := userKeys.GetSpendingKeyPair()
	if err != nil {
		return "", err
	}

	u, err := utxo.NewUtxo(types.UtxoParams{
		Amount:            big.NewInt(0),
		Erc20TokenAddress: constants.ZeroAddress,
		NullifyingKey:     nullifyingKey,
		SpendingPublicKey: []*big.Int{spendingKeyPair.PubSpendingBJJPoint[0], spendingKeyPair.PubSpendingBJJPoint[1]},
		IsNewStyle:        true,
	})
	if err != nil {
		return "", err
	}
	if u.H0 == nil {
		return "", errors.New("getRecipientInfoFromUserKeys: H0 is not set")
	}
	stealthAddress, err := u.GetStealthAddress()
	if err != nil {
		return "", err
	}
	h1, err := cryptokeys.GetH1FromH0(*u.H0, nullifyingKey)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s,%s,%s,%s,%s", stealthAddress, u.H0[0].String(), u.H0[1].String(), h1[1].String(), encPair.PublicKey), nil
}

func GetStealthAddressStructureFromUserKeys(userKeys *cryptokeys.UserKeys) (types.StealthAddressStructure, error) {
	recipientInfo, err := GetRecipientInfoFromUserKeys(userKeys)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	return ConstructStealthAddressStructure(recipientInfo)
}
