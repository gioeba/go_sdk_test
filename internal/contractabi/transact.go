package contractabi

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/types"
)

type abiDimensions struct {
	TokenNumber     uint16
	NullifierAmount uint16
	OutputAmount    uint16
}

type abiFeeStructure struct {
	FeeToken     common.Address
	FlatFee      *big.Int
	VariableRate *big.Int
}

type abiStealthAddressStructure struct {
	ExtraRandomization *big.Int
	StealthAddress     *big.Int
	H0                 *big.Int
	H1                 *big.Int
}

type abiUseApprovalUTXOData struct {
	ApprovalChanges           []*big.Int
	ExternalApprovalAddresses []common.Address
	ConversionInHinkalAddress []*big.Int
}

type abiHinkalLogicArgs struct {
	HinkalLogicAction      *big.Int
	InHinkalAddress        *big.Int
	ExecuteApprovalChanges []*big.Int
	DoPreTxApproval        bool
	UseApprovalUtxoData    []abiUseApprovalUTXOData
}

type abiHookData struct {
	PreHookContract  common.Address
	HookContract     common.Address
	PreHookMetadata  []byte
	PostHookMetadata []byte
}

type abiSignatureData struct {
	V               uint8
	R               [32]byte
	S               [32]byte
	AccessKey       *big.Int
	Nonce           *big.Int
	EthereumAddress common.Address
}

type abiCircomData struct {
	RootHashHinkal          *big.Int
	Erc20TokenAddresses     []common.Address
	TokenIDs                []*big.Int `abi:"tokenIds"`
	AmountChanges           []*big.Int
	OnChainCreation         []bool
	SlippageValues          []*big.Int
	InputNullifiers         [][]*big.Int
	OutCommitments          [][]*big.Int
	EncryptedOutputs        [][][]byte
	FeeStructure            abiFeeStructure
	TimeStamp               *big.Int
	StealthAddressStructure abiStealthAddressStructure
	RootHashAccessToken     *big.Int
	CalldataHash            *big.Int
	PublicSignalCount       uint16
	Relay                   common.Address
	ExternalAddress         common.Address
	ExternalActionID        *big.Int `abi:"externalActionId"`
	ExternalActionMetadata  []byte
	HinkalLogicArgs         abiHinkalLogicArgs
	HookData                abiHookData
	SignatureData           abiSignatureData
	OriginalSender          common.Address
}

// abiProofSignature is the Tron-only proofSignature argument prepended to transact.
type abiProofSignature struct {
	V uint8
	R [32]byte
	S [32]byte
}

func zeroIfNil(n *big.Int) *big.Int {
	if n == nil {
		return big.NewInt(0)
	}
	return n
}

func parseBigSlice(values []string) ([]*big.Int, error) {
	out := make([]*big.Int, len(values))
	for i, v := range values {
		n, err := utils.ParseBigInt(v)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

func parseBigMatrix(values [][]string) ([][]*big.Int, error) {
	out := make([][]*big.Int, len(values))
	for i, inner := range values {
		parsed, err := parseBigSlice(inner)
		if err != nil {
			return nil, err
		}
		out[i] = parsed
	}
	return out, nil
}

func hexBytesMatrix(values [][]string) [][][]byte {
	out := make([][][]byte, len(values))
	for i, inner := range values {
		out[i] = make([][]byte, len(inner))
		for j, v := range inner {
			out[i][j] = common.FromHex(v)
		}
	}
	return out
}

func toAddresses(values []string) []common.Address {
	out := make([]common.Address, len(values))
	for i, v := range values {
		out[i] = common.HexToAddress(v)
	}
	return out
}

// BytesToBytes32 right-aligns hex value into a 32-byte array.
func BytesToBytes32(value string) [32]byte {
	var out [32]byte
	b := common.FromHex(value)
	if len(b) > 32 {
		b = b[len(b)-32:]
	}
	copy(out[32-len(b):], b)
	return out
}

func signatureDataToABI(s types.SignatureData) (abiSignatureData, error) {
	v, err := utils.ParseBigInt(s.V)
	if err != nil {
		return abiSignatureData{}, fmt.Errorf("signatureData.v: %w", err)
	}
	accessKey, err := utils.ParseBigInt(s.AccessKey)
	if err != nil {
		return abiSignatureData{}, fmt.Errorf("signatureData.accessKey: %w", err)
	}
	return abiSignatureData{
		V:               uint8(v.Uint64()),
		R:               BytesToBytes32(s.R),
		S:               BytesToBytes32(s.S),
		AccessKey:       accessKey,
		Nonce:           big.NewInt(int64(s.Nonce)),
		EthereumAddress: common.HexToAddress(s.EthereumAddress),
	}, nil
}

func circomDataToABI(c types.CircomDataType) (abiCircomData, error) {
	tokenIDs, err := parseBigSlice(c.TokenIDs)
	if err != nil {
		return abiCircomData{}, err
	}
	inputNullifiers, err := parseBigMatrix(c.InputNullifiers)
	if err != nil {
		return abiCircomData{}, err
	}
	outCommitments, err := parseBigMatrix(c.OutCommitments)
	if err != nil {
		return abiCircomData{}, err
	}
	timeStamp := big.NewInt(0)
	if c.TimeStamp != "" {
		timeStamp, err = utils.ParseBigInt(c.TimeStamp)
		if err != nil {
			return abiCircomData{}, err
		}
	}
	signatureData, err := signatureDataToABI(c.SignatureData)
	if err != nil {
		return abiCircomData{}, err
	}

	useApprovalUtxoData := make([]abiUseApprovalUTXOData, len(c.HinkalLogicArgs.UseApprovalUtxoData))
	for i, d := range c.HinkalLogicArgs.UseApprovalUtxoData {
		useApprovalUtxoData[i] = abiUseApprovalUTXOData{
			ApprovalChanges:           d.ApprovalChanges,
			ExternalApprovalAddresses: toAddresses(d.ExternalApprovalAddresses),
			ConversionInHinkalAddress: d.ConversionInHinkalAddress,
		}
	}

	return abiCircomData{
		RootHashHinkal:      zeroIfNil(c.RootHashHinkal),
		Erc20TokenAddresses: toAddresses(c.Erc20TokenAddresses),
		TokenIDs:            tokenIDs,
		AmountChanges:       c.AmountChanges,
		OnChainCreation:     c.OnChainCreation,
		SlippageValues:      c.SlippageValues,
		InputNullifiers:     inputNullifiers,
		OutCommitments:      outCommitments,
		EncryptedOutputs:    hexBytesMatrix(c.EncryptedOutputs),
		FeeStructure: abiFeeStructure{
			FeeToken:     common.HexToAddress(c.FeeStructure.FeeToken),
			FlatFee:      c.FeeStructure.FlatFee,
			VariableRate: c.FeeStructure.VariableRate,
		},
		TimeStamp: timeStamp,
		StealthAddressStructure: abiStealthAddressStructure{
			ExtraRandomization: c.StealthAddressStructure.ExtraRandomization,
			StealthAddress:     c.StealthAddressStructure.StealthAddress,
			H0:                 c.StealthAddressStructure.H0,
			H1:                 c.StealthAddressStructure.H1,
		},
		RootHashAccessToken:    zeroIfNil(c.RootHashAccessToken),
		CalldataHash:           zeroIfNil(c.CalldataHash),
		PublicSignalCount:      uint16(c.PublicSignalCount),
		Relay:                  common.HexToAddress(c.Relay),
		ExternalAddress:        common.HexToAddress(c.ExternalAddress),
		ExternalActionID:       zeroIfNil(c.ExternalActionID),
		ExternalActionMetadata: common.FromHex(c.ExternalActionMetadata),
		HinkalLogicArgs: abiHinkalLogicArgs{
			HinkalLogicAction:      big.NewInt(int64(c.HinkalLogicArgs.HinkalLogicAction)),
			InHinkalAddress:        zeroIfNil(c.HinkalLogicArgs.InHinkalAddress),
			ExecuteApprovalChanges: c.HinkalLogicArgs.ExecuteApprovalChanges,
			DoPreTxApproval:        c.HinkalLogicArgs.DoPreTxApproval,
			UseApprovalUtxoData:    useApprovalUtxoData,
		},
		HookData: abiHookData{
			PreHookContract:  common.HexToAddress(c.HookData.PreHookContract),
			HookContract:     common.HexToAddress(c.HookData.HookContract),
			PreHookMetadata:  common.FromHex(c.HookData.PreHookMetadata),
			PostHookMetadata: common.FromHex(c.HookData.PostHookMetadata),
		},
		SignatureData:  signatureData,
		OriginalSender: common.HexToAddress(c.OriginalSender),
	}, nil
}

func zkCallDataToABI(zk types.NewZkCallDataType) (a [2]*big.Int, b [2][2]*big.Int, c [2]*big.Int, err error) {
	parse := utils.ParseBigInt
	for i := 0; i < 2; i++ {
		if a[i], err = parse(zk.A[i]); err != nil {
			return
		}
		if c[i], err = parse(zk.C[i]); err != nil {
			return
		}
		for j := 0; j < 2; j++ {
			if b[i][j], err = parse(zk.B[i][j]); err != nil {
				return
			}
		}
	}
	return
}

func toABIDimensions(d types.DimDataType) abiDimensions {
	return abiDimensions{
		TokenNumber:     uint16(d.TokenNumber),
		NullifierAmount: uint16(d.NullifierAmount),
		OutputAmount:    uint16(d.OutputAmount),
	}
}

// PackTransact ABI-encodes a call to the EVM Hinkal contract's transact method.
func PackTransact(chainID int, zkCallData types.NewZkCallDataType, dimData types.DimDataType, circomData types.CircomDataType) ([]byte, error) {
	hinkalABI, err := Hinkal(chainID)
	if err != nil {
		return nil, err
	}
	a, b, c, err := zkCallDataToABI(zkCallData)
	if err != nil {
		return nil, err
	}
	circom, err := circomDataToABI(circomData)
	if err != nil {
		return nil, err
	}
	return hinkalABI.Pack("transact", a, b, c, toABIDimensions(dimData), circom)
}

// PackTronTransact ABI-encodes a call to the Tron Hinkal contract's transact method, which prepends
// a proofSignature argument.
func PackTronTransact(
	chainID int,
	proofV uint8,
	proofR, proofS [32]byte,
	zkCallData types.NewZkCallDataType,
	dimData types.DimDataType,
	circomData types.CircomDataType,
) ([]byte, error) {
	hinkalABI, err := Hinkal(chainID)
	if err != nil {
		return nil, err
	}
	a, b, c, err := zkCallDataToABI(zkCallData)
	if err != nil {
		return nil, err
	}
	circom, err := circomDataToABI(circomData)
	if err != nil {
		return nil, err
	}
	proof := abiProofSignature{V: proofV, R: proofR, S: proofS}
	return hinkalABI.Pack("transact", proof, a, b, c, toABIDimensions(dimData), circom)
}
