package snarkjs

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/types"
)

type hookTuple struct {
	P0 common.Address
	P1 common.Address
	P2 []byte
	P3 []byte
}

type feeTuple struct {
	P0 common.Address
	P1 *big.Int
	P2 *big.Int
}

type sigTuple struct {
	V               uint8
	R               [32]byte
	S               [32]byte
	AccessKey       *big.Int
	Nonce           *big.Int
	EthereumAddress common.Address
}

func hookDataValues(hookData *types.HookDataType) hookTuple {
	if hookData == nil {
		d := types.DefaultHookData()
		return hookTuple{
			P0: common.HexToAddress(d.PreHookContract),
			P1: common.HexToAddress(d.HookContract),
			P2: common.FromHex(d.PreHookMetadata),
			P3: common.FromHex(d.PostHookMetadata),
		}
	}
	return hookTuple{
		P0: common.HexToAddress(hookData.HookContract),
		P1: common.HexToAddress(hookData.PreHookContract),
		P2: common.FromHex(hookData.PreHookMetadata),
		P3: common.FromHex(hookData.PostHookMetadata),
	}
}

func toBytes32(value string) [32]byte {
	var out [32]byte
	b := common.FromHex(value)
	if len(b) > 32 {
		b = b[len(b)-32:]
	}
	copy(out[32-len(b):], b)
	return out
}

func signatureDataToTuple(s types.SignatureData) (sigTuple, error) {
	v, err := utils.ParseBigInt(s.V)
	if err != nil {
		return sigTuple{}, fmt.Errorf("signatureData.v: %w", err)
	}
	accessKey, err := utils.ParseBigInt(s.AccessKey)
	if err != nil {
		return sigTuple{}, fmt.Errorf("signatureData.accessKey: %w", err)
	}
	return sigTuple{
		V:               uint8(v.Uint64()),
		R:               toBytes32(s.R),
		S:               toBytes32(s.S),
		AccessKey:       accessKey,
		Nonce:           big.NewInt(int64(s.Nonce)),
		EthereumAddress: common.HexToAddress(s.EthereumAddress),
	}, nil
}

func encryptedOutputsToBytes(encryptedOutputs [][]string) [][][]byte {
	out := make([][][]byte, len(encryptedOutputs))
	for i, inner := range encryptedOutputs {
		out[i] = make([][]byte, len(inner))
		for j, v := range inner {
			out[i][j] = common.FromHex(v)
		}
	}
	return out
}

func CreateCallDataHash(
	publicSignalCount int,
	relay string,
	externalAddress string,
	externalActionID types.ExternalActionID,
	externalActionMetadata string,
	encryptedOutputs [][]string,
	hookData *types.HookDataType,
	slippageValues []*big.Int,
	onChainCreation []bool,
	feeStructure types.FeeStructure,
	signatureData types.SignatureData,
	originalSender string,
) (*big.Int, error) {
	if originalSender == "" {
		originalSender = GetOriginalSender(externalAddress, relay)
	}
	if externalAddress == "" {
		externalAddress = constants.ZeroAddress
	}

	encodedValues1, err := (abi.Arguments{
		{Type: abiUint16}, {Type: abiAddress}, {Type: abiAddress}, {Type: abiUint256}, {Type: abiBytes},
	}).Pack(
		uint16(publicSignalCount),
		common.HexToAddress(relay),
		common.HexToAddress(externalAddress),
		GetExternalActionIDHash(externalActionID),
		common.FromHex(externalActionMetadata),
	)
	if err != nil {
		return nil, fmt.Errorf("calldata encode 1: %w", err)
	}

	sig, err := signatureDataToTuple(signatureData)
	if err != nil {
		return nil, err
	}

	encodedValues2, err := (abi.Arguments{
		{Type: abiHookTuple}, {Type: abiBytesArr2}, {Type: abiFeeTuple}, {Type: abiInt256Arr}, {Type: abiBoolArr}, {Type: abiSigTuple}, {Type: abiAddress},
	}).Pack(
		hookDataValues(hookData),
		encryptedOutputsToBytes(encryptedOutputs),
		feeTuple{P0: common.HexToAddress(feeStructure.FeeToken), P1: feeStructure.FlatFee, P2: feeStructure.VariableRate},
		slippageValues,
		onChainCreation,
		sig,
		common.HexToAddress(originalSender),
	)
	if err != nil {
		return nil, fmt.Errorf("calldata encode 2: %w", err)
	}

	calldataHash1 := new(big.Int).SetBytes(gethcrypto.Keccak256(encodedValues1))
	calldataHash2 := new(big.Int).SetBytes(gethcrypto.Keccak256(encodedValues2))

	encodedValues, err := (abi.Arguments{{Type: abiUint256}, {Type: abiUint256}}).Pack(calldataHash1, calldataHash2)
	if err != nil {
		return nil, fmt.Errorf("calldata encode final: %w", err)
	}

	calldataHash := new(big.Int).SetBytes(gethcrypto.Keccak256(encodedValues))
	return calldataHash.Mod(calldataHash, circomP), nil
}
