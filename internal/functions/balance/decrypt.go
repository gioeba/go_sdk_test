package balance

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mr-tron/base58"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/crypto"
	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

const sealedKeysVersion = 54912

var (
	highestBit  = new(big.Int).Lsh(big.NewInt(1), 255)
	highestMask = new(big.Int).Sub(new(big.Int).Set(highestBit), big.NewInt(1))
)

func SealedKeysPrefix() []byte {
	return []byte{byte(sealedKeysVersion >> 8), byte(sealedKeysVersion & 0xFF)}
}

func DecryptUtxo(encryptedData []byte, uk *cryptokeys.UserKeys) (*utxo.Utxo, error) {
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}
	sk, pk, err := cryptokeys.EncryptionKeyPair(spk)
	if err != nil {
		return nil, err
	}

	prefix := SealedKeysPrefix()
	var plain []byte
	if bytes.HasPrefix(encryptedData, prefix) {
		plain, err = cryptokeys.DecryptSealedKeys(encryptedData[len(prefix):], &pk, &sk, 0)
	} else {
		plain, err = cryptokeys.DecryptBoxSeal(encryptedData, &pk, &sk)
	}
	if err != nil {
		return nil, err
	}

	parts := splitHexParts(string(plain))
	if len(parts) < 5 {
		return nil, errors.New("decrypt utxo: too few fields")
	}

	amount, err := utils.ParseBigInt("0x" + parts[0])
	if err != nil {
		return nil, err
	}
	randomization, err := utils.ParseBigInt("0x" + parts[2])
	if err != nil {
		return nil, err
	}
	ts, err := utils.ParseBigInt("0x" + parts[4])
	if err != nil {
		return nil, err
	}

	isNewLayout := len(parts) >= 8
	isNewStyle := false
	if isNewLayout {
		flag, err := utils.ParseBigInt("0x" + parts[5])
		if err != nil {
			return nil, err
		}
		isNewStyle = flag.Sign() != 0
	}

	mintIndex := 6
	if isNewLayout {
		mintIndex = 8
	}
	mint := ""
	if mintIndex < len(parts) {
		mintBytes, err := hex.DecodeString(parts[mintIndex])
		if err == nil && len(mintBytes) == 32 {
			mint = base58.Encode(mintBytes)
		}
	}

	params := types.UtxoParams{
		Amount:            amount,
		Erc20TokenAddress: "0x" + parts[1],
		MintAddress:       mint,
		TimeStamp:         ts.String(),
		NullifyingKey:     spk,
		StealthAddress:    "0x" + parts[3],
		Randomization:     randomization,
		IsNewStyle:        isNewStyle,
	}
	if isNewStyle {
		h0x, err := utils.ParseBigInt("0x" + parts[6])
		if err != nil {
			return nil, err
		}
		h0y, err := utils.ParseBigInt("0x" + parts[7])
		if err != nil {
			return nil, err
		}
		params.H0 = &types.JubPoint{h0x, h0y}
	}
	return utxo.NewUtxo(params)
}

func DecryptUtxoHex(encryptedHex string, uk *cryptokeys.UserKeys) (*utxo.Utxo, error) {
	return DecryptUtxo(common.FromHex(encryptedHex), uk)
}

func DecodeUtxo(encodedOutput string, uk *cryptokeys.UserKeys, chainID int) (*utxo.Utxo, error) {
	if constants.IsSolanaLike(chainID) {
		return DecodeSolanaOnChainUtxo(encodedOutput, uk)
	}
	return DecodeEvmUtxoHex(encodedOutput, uk)
}

func DecodeEvmUtxo(encodedData []byte, uk *cryptokeys.UserKeys) (*utxo.Utxo, error) {
	if len(encodedData) != 32*8 {
		return nil, fmt.Errorf("decode evm utxo: expected %d bytes, got %d", 32*8, len(encodedData))
	}
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}

	word := func(i int) *big.Int {
		return new(big.Int).SetBytes(encodedData[i*32 : (i+1)*32])
	}
	amount := word(0)
	tokenAddress := common.BytesToAddress(encodedData[32+12 : 64]).Hex()
	rawH0x := word(2)
	isNewStyle := rawH0x.Bit(255) == 1
	randomization := new(big.Int).And(rawH0x, highestMask)
	stealthAddress := utils.ToBeHex(word(3))
	h0y := word(4)
	h1y := word(5)
	ts := word(6)

	ok, err := checkUtxoSignature(randomization, h0y, h1y, spk, isNewStyle)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("decode evm utxo: stealth pair does not belong to user")
	}

	params := types.UtxoParams{
		Amount:            amount,
		Erc20TokenAddress: tokenAddress,
		TimeStamp:         ts.String(),
		NullifyingKey:     spk,
		StealthAddress:    stealthAddress,
		IsNewStyle:        isNewStyle,
	}
	if isNewStyle {
		params.H0 = &types.JubPoint{randomization, h0y}
	} else {
		params.Randomization = randomization
	}
	return utxo.NewUtxo(params)
}

func DecodeEvmUtxoHex(encodedHex string, uk *cryptokeys.UserKeys) (*utxo.Utxo, error) {
	return DecodeEvmUtxo(common.FromHex(encodedHex), uk)
}

func DecodeSolanaOnChainUtxo(encodedOutput string, uk *cryptokeys.UserKeys) (*utxo.Utxo, error) {
	const prefix = "solana-on-chain-utxo:"
	if !strings.HasPrefix(encodedOutput, prefix) {
		return nil, errors.New("decode solana on-chain utxo: invalid prefix")
	}
	encodedData := common.FromHex(strings.TrimPrefix(encodedOutput, prefix))
	if len(encodedData) != 32*8 {
		return nil, fmt.Errorf("decode solana on-chain utxo: expected %d bytes, got %d", 32*8, len(encodedData))
	}
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		return nil, err
	}

	wordBytes := func(i int) []byte {
		return encodedData[i*32 : (i+1)*32]
	}
	word := func(i int) *big.Int {
		return new(big.Int).SetBytes(wordBytes(i))
	}
	amount := word(0)
	mintPart1 := word(1)
	mintPart2 := word(2)
	rawH0x := word(3)
	isNewStyle := rawH0x.Bit(255) == 1
	randomization := new(big.Int).And(rawH0x, highestMask)
	stealthAddress := utils.ToBeHex(word(4))
	h0y := word(5)
	h1y := word(6)
	ts := word(7)

	ok, err := checkUtxoSignature(randomization, h0y, h1y, spk, isNewStyle)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("decode solana on-chain utxo: stealth pair does not belong to user")
	}

	mintBytes := make([]byte, 0, 32)
	mintBytes = append(mintBytes, wordBytes(1)[16:]...)
	mintBytes = append(mintBytes, wordBytes(2)[16:]...)
	tokenHash, err := crypto.PoseidonBig(mintPart1, mintPart2)
	if err != nil {
		return nil, err
	}

	params := types.UtxoParams{
		Amount:            amount,
		Erc20TokenAddress: utils.ToBeHex(tokenHash),
		MintAddress:       base58.Encode(mintBytes),
		TimeStamp:         ts.String(),
		NullifyingKey:     spk,
		StealthAddress:    stealthAddress,
		IsNewStyle:        isNewStyle,
	}
	if isNewStyle {
		params.H0 = &types.JubPoint{randomization, h0y}
	} else {
		params.Randomization = randomization
	}
	return utxo.NewUtxo(params)
}

func checkUtxoSignature(h0x, h0y, h1y *big.Int, privateKey string, isNewStyle bool) (bool, error) {
	if isNewStyle {
		return cryptokeys.VerifyStealthPair(
			types.JubPoint{new(big.Int).Set(h0x), new(big.Int).Set(h0y)},
			types.JubPoint{new(big.Int), new(big.Int).Set(h1y)},
			privateKey,
			true,
		)
	}
	return cryptokeys.CheckSignature(h0x, h0y, h1y, privateKey)
}

func splitHexParts(s string) []string {
	raw := strings.Split(s, "0x")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
