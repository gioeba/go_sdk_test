package types

import "math/big"

const (
	zeroAddress     = "0x0000000000000000000000000000000000000000"
	defaultFeeToken = zeroAddress
)

type FeeStructure struct {
	FeeToken     string   `json:"feeToken"`
	FlatFee      *big.Int `json:"flatFee"`
	VariableRate *big.Int `json:"variableRate"` // beeps = 0.01 of 1%
}

type FeeStructureJSON struct {
	FeeToken     string `json:"feeToken"`
	FlatFee      string `json:"flatFee"`
	VariableRate string `json:"variableRate"`
}

func ZeroFeeStructure() FeeStructure {
	return FeeStructure{
		FeeToken:     defaultFeeToken,
		FlatFee:      big.NewInt(0),
		VariableRate: big.NewInt(0),
	}
}

type HinkalLogicAction int

const (
	HinkalLogicActionNone HinkalLogicAction = iota
	HinkalLogicActionApprove
	HinkalLogicActionReleaseBuffer
	HinkalLogicActionExecute
)

type UseApprovalUTXOData struct {
	ApprovalChanges           []*big.Int `json:"approvalChanges"`
	ExternalApprovalAddresses []string   `json:"externalApprovalAddresses"`
	ConversionInHinkalAddress []*big.Int `json:"conversionInHinkalAddress"`
}

type UseApprovalUTXODataJSON struct {
	ApprovalChanges           []string `json:"approvalChanges"`
	ExternalApprovalAddresses []string `json:"externalApprovalAddresses"`
	ConversionInHinkalAddress []string `json:"conversionInHinkalAddress"`
}

type HinkalLogicArgs struct {
	HinkalLogicAction      HinkalLogicAction     `json:"hinkalLogicAction"`
	ExecuteApprovalChanges []*big.Int            `json:"executeApprovalChanges"`
	DoPreTxApproval        bool                  `json:"doPreTxApproval"`
	InHinkalAddress        *big.Int              `json:"inHinkalAddress"`
	UseApprovalUtxoData    []UseApprovalUTXOData `json:"useApprovalUtxoData"`
	InteractionAddress     string                `json:"interactionAddress,omitempty"`
}

type HinkalLogicArgsJSON struct {
	HinkalLogicAction      HinkalLogicAction         `json:"hinkalLogicAction"`
	ExecuteApprovalChanges []string                  `json:"executeApprovalChanges"`
	DoPreTxApproval        bool                      `json:"doPreTxApproval"`
	InHinkalAddress        string                    `json:"inHinkalAddress"`
	UseApprovalUtxoData    []UseApprovalUTXODataJSON `json:"useApprovalUtxoData"`
	InteractionAddress     string                    `json:"interactionAddress,omitempty"`
}

func DefaultHinkalLogicArgs(count int) HinkalLogicArgs {
	executeApprovalChanges := make([]*big.Int, count)
	useApprovalUtxoData := make([]UseApprovalUTXOData, count)
	for i := 0; i < count; i++ {
		executeApprovalChanges[i] = big.NewInt(0)
		useApprovalUtxoData[i] = UseApprovalUTXOData{
			ApprovalChanges:           []*big.Int{big.NewInt(0)},
			ExternalApprovalAddresses: []string{zeroAddress},
			ConversionInHinkalAddress: []*big.Int{big.NewInt(0)},
		}
	}
	return HinkalLogicArgs{
		HinkalLogicAction:      HinkalLogicActionNone,
		ExecuteApprovalChanges: executeApprovalChanges,
		DoPreTxApproval:        false,
		InHinkalAddress:        big.NewInt(0),
		UseApprovalUtxoData:    useApprovalUtxoData,
	}
}

// HinkalConfig holds configuration options for a Hinkal instance.
type HinkalConfig struct {
	GenerateProofRemotely    *bool
	DisableMerkleTreeUpdates bool
	CacheDevice              ICacheDevice
	CacheFilePath            string
	UseFileCache             bool
	DisableCaching           bool
	SerializedCache          map[string]string
	TronChainOverride        int
}

// LoginMessageMode selects which message the user signs to derive their UserKeys.
type LoginMessageMode int

const (
	LoginMessageModeProtocol LoginMessageMode = iota
	LoginMessageModePrivateTransfer
)

const (
	SigningMessage                = "Login to Hinkal Protocol"
	PrivateTransferSigningMessage = "Login to Hinkal's Private Transfer App"
)
