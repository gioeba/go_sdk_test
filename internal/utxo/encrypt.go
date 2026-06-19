package utxo

import (
	"encoding/hex"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mr-tron/base58"

	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
)

// 2-byte big-endian version prefix marking the sealed-keys (multi-recipient) format.
const sealedKeysVersion = 54912

func sealedKeysPrefix() []byte {
	return []byte{byte(sealedKeysVersion >> 8), byte(sealedKeysVersion & 0xFF)}
}

func ensure0x(value string) string {
	if strings.HasPrefix(value, "0x") {
		return value
	}
	return "0x" + value
}

func EncryptUtxo(u *Utxo) ([]byte, error) {
	stealth, err := u.GetStealthAddress()
	if err != nil {
		return nil, err
	}
	ts, err := utils.ParseBigInt(u.TimeStamp)
	if err != nil {
		return nil, err
	}

	randomization := u.Randomization
	if randomization == nil {
		randomization = big.NewInt(0)
	}
	isNewStyle := big.NewInt(0)
	if u.IsNewStyle {
		isNewStyle = big.NewInt(1)
	}
	h0x, h0y := big.NewInt(0), big.NewInt(0)
	if u.H0 != nil {
		h0x, h0y = (*u.H0)[0], (*u.H0)[1]
	}

	// converting data to Uint8Array; all data is in hex format
	parts := [][]byte{
		[]byte(utils.ToBeHex(u.Amount)),
		[]byte(ensure0x(u.Erc20TokenAddress)),
		[]byte(utils.ToBeHex(randomization)),
		[]byte(ensure0x(stealth)),
		[]byte(utils.ToBeHex(ts)),
		[]byte(utils.ToBeHex(isNewStyle)), // previously was tokenId now is isNewStyle flag
		[]byte(utils.ToBeHex(h0x)),        // point H0[0]
		[]byte(utils.ToBeHex(h0y)),        // point H0[1]
	}
	if u.MintAddress != "" {
		mintBytes, err := base58.Decode(u.MintAddress)
		if err != nil {
			return nil, err
		}
		parts = append(parts, []byte("0x"+hex.EncodeToString(mintBytes)))
	}

	var buf []byte
	for _, p := range parts {
		buf = append(buf, p...)
	}

	encKeyHex, err := u.GetEncryptionKey()
	if err != nil {
		return nil, err
	}
	pkBytes := common.FromHex(encKeyHex)
	if len(pkBytes) != 32 {
		return nil, errors.New("utxo: encryption public key is not 32 bytes")
	}
	var pk [32]byte
	copy(pk[:], pkBytes)

	// encrypting with encryptionPublicKey, prefixed with the version marker
	encrypted, err := cryptokeys.EncryptSealedKeys(buf, []*[32]byte{&pk})
	if err != nil {
		return nil, err
	}
	return append(sealedKeysPrefix(), encrypted...), nil
}
