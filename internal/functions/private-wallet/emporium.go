package privatewallet

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	"github.com/gioeba/go_sdk_test/constants"
)

type emporiumMetadataTuple struct {
	V             uint8
	R             [32]byte
	S             [32]byte
	Message       *big.Int
	SignerAddress common.Address
	Ops           [][]byte
}

func EncodeEmporiumMetadata(
	chainID int,
	emporiumAddress string,
	privateKey string,
	ops []string,
	message *big.Int,
	signerAddress string,
) (string, error) {
	if message == nil {
		message = big.NewInt(0)
	}
	if signerAddress == "" {
		signerAddress = constants.ZeroAddress
	}

	var v uint8
	var r, s [32]byte
	if privateKey != "" {
		var err error
		v, r, s, err = signEmporium(chainID, emporiumAddress, privateKey, ops, message)
		if err != nil {
			return "", err
		}
	}

	opsBytes := make([][]byte, len(ops))
	for i, op := range ops {
		opsBytes[i] = common.FromHex(op)
	}

	tupleType, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "v", Type: "uint8"},
		{Name: "r", Type: "bytes32"},
		{Name: "s", Type: "bytes32"},
		{Name: "message", Type: "uint256"},
		{Name: "signerAddress", Type: "address"},
		{Name: "ops", Type: "bytes[]"},
	})
	if err != nil {
		return "", fmt.Errorf("emporium metadata tuple type: %w", err)
	}

	packed, err := (abi.Arguments{{Type: tupleType}}).Pack(emporiumMetadataTuple{
		V:             v,
		R:             r,
		S:             s,
		Message:       message,
		SignerAddress: common.HexToAddress(signerAddress),
		Ops:           opsBytes,
	})
	if err != nil {
		return "", fmt.Errorf("emporium metadata pack: %w", err)
	}

	return "0x" + hex.EncodeToString(packed), nil
}

func signEmporium(chainID int, emporiumAddress, privateKey string, ops []string, message *big.Int) (uint8, [32]byte, [32]byte, error) {
	var r, s [32]byte
	if constants.IsSolanaLike(chainID) {
		return 0, r, s, errors.New("privatewallet: solana does not support typed data")
	}

	key, err := gethcrypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		return 0, r, s, fmt.Errorf("emporium signer key: %w", err)
	}

	opsValues := make([]interface{}, len(ops))
	for i, op := range ops {
		opsValues[i] = common.FromHex(op)
	}

	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"EmporiumSignature": {
				{Name: "message", Type: "uint256"},
				{Name: "ops", Type: "bytes[]"},
			},
		},
		PrimaryType: "EmporiumSignature",
		Domain: apitypes.TypedDataDomain{
			Name:              "Emporium",
			Version:           "1.0.0",
			ChainId:           math.NewHexOrDecimal256(int64(chainID)),
			VerifyingContract: emporiumAddress,
		},
		Message: apitypes.TypedDataMessage{
			"message": message.String(),
			"ops":     opsValues,
		},
	}

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return 0, r, s, err
	}
	structHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return 0, r, s, err
	}

	raw := append([]byte{0x19, 0x01}, domainSeparator...)
	raw = append(raw, structHash...)
	digest := gethcrypto.Keccak256(raw)

	sig, err := gethcrypto.Sign(digest, key)
	if err != nil {
		return 0, r, s, err
	}
	copy(r[:], sig[0:32])
	copy(s[:], sig[32:64])
	return sig[64] + 27, r, s, nil
}

func SignerAddressFromPrivateKey(chainID int, privateKey string) (string, error) {
	if constants.IsSolanaLike(chainID) {
		return "", errors.New("privatewallet: solana signer address derivation not implemented")
	}
	key, err := gethcrypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		return "", fmt.Errorf("signer key: %w", err)
	}
	return gethcrypto.PubkeyToAddress(key.PublicKey).Hex(), nil
}
