package types

import "math/big"

type DimDataType struct {
	TokenNumber     int `json:"tokenNumber"`
	NullifierAmount int `json:"nullifierAmount"`
	OutputAmount    int `json:"outputAmount"`
}

type HookDataType struct {
	HookContract     string `json:"hookContract"`
	PreHookContract  string `json:"preHookContract"`
	PreHookMetadata  string `json:"preHookMetadata"`
	PostHookMetadata string `json:"postHookMetadata"`
}

func DefaultHookData() HookDataType {
	return HookDataType{
		PreHookContract:  zeroAddress,
		HookContract:     zeroAddress,
		PreHookMetadata:  "0x00",
		PostHookMetadata: "0x00",
	}
}

type StealthAddressStructure struct {
	ExtraRandomization *big.Int `json:"extraRandomization"`
	StealthAddress     *big.Int `json:"stealthAddress"`
	H0                 *big.Int `json:"H0"`
	H1                 *big.Int `json:"H1"`
}

type StealthAddressStructureJSON struct {
	ExtraRandomization string `json:"extraRandomization"`
	StealthAddress     string `json:"stealthAddress"`
	H0                 string `json:"H0"`
	H1                 string `json:"H1"`
}

func DefaultStealthAddressStructure() StealthAddressStructure {
	return StealthAddressStructure{
		ExtraRandomization: big.NewInt(1),
		StealthAddress:     big.NewInt(0),
		H0:                 big.NewInt(0),
		H1:                 big.NewInt(0),
	}
}

type CircomDataType struct {
	RootHashHinkal          *big.Int                `json:"rootHashHinkal"`
	Erc20TokenAddresses     []string                `json:"erc20TokenAddresses"`
	TokenIDs                []string                `json:"tokenIds"`
	AmountChanges           []*big.Int              `json:"amountChanges"`
	InputNullifiers         [][]string              `json:"inputNullifiers"`
	OutCommitments          [][]string              `json:"outCommitments"`
	EncryptedOutputs        [][]string              `json:"encryptedOutputs"`
	TimeStamp               string                  `json:"timeStamp,omitempty"`
	StealthAddressStructure StealthAddressStructure `json:"stealthAddressStructure"`
	RootHashAccessToken     *big.Int                `json:"rootHashAccessToken"`
	Relay                   string                  `json:"relay"`
	ExternalAddress         string                  `json:"externalAddress"`
	ExternalActionMetadata  string                  `json:"externalActionMetadata"`
	ExternalActionID        *big.Int                `json:"externalActionId"`
	HookData                HookDataType            `json:"hookData"`
	CalldataHash            *big.Int                `json:"calldataHash"`
	PublicSignalCount       int                     `json:"publicSignalCount"`
	OnChainCreation         []bool                  `json:"onChainCreation"`
	SlippageValues          []*big.Int              `json:"slippageValues"`
	HinkalLogicArgs         HinkalLogicArgs         `json:"hinkalLogicArgs"`
	FeeStructure            FeeStructure            `json:"feeStructure"`
	SignatureData           SignatureData           `json:"signatureData"`
	OriginalSender          string                  `json:"originalSender"`
}

type CircomDataJSONType struct {
	RootHashHinkal          *string                     `json:"rootHashHinkal,omitempty"`
	Erc20TokenAddresses     []string                    `json:"erc20TokenAddresses"`
	TokenIDs                []string                    `json:"tokenIds"`
	AmountChanges           []string                    `json:"amountChanges"`
	InputNullifiers         [][]string                  `json:"inputNullifiers"`
	OutCommitments          [][]string                  `json:"outCommitments"`
	EncryptedOutputs        [][]string                  `json:"encryptedOutputs"`
	StealthAddressStructure StealthAddressStructureJSON `json:"stealthAddressStructure"`
	TimeStamp               string                      `json:"timeStamp,omitempty"`
	RootHashAccessToken     *string                     `json:"rootHashAccessToken,omitempty"`
	Relay                   string                      `json:"relay"`
	ExternalAddress         string                      `json:"externalAddress"`
	ExternalActionMetadata  string                      `json:"externalActionMetadata"`
	ExternalActionID        string                      `json:"externalActionId"`
	HookData                HookDataType                `json:"hookData"`
	CalldataHash            string                      `json:"calldataHash"`
	PublicSignalCount       int                         `json:"publicSignalCount"`
	OnChainCreation         []bool                      `json:"onChainCreation"`
	SlippageValues          []string                    `json:"slippageValues"`
	HinkalLogicArgs         HinkalLogicArgsJSON         `json:"hinkalLogicArgs"`
	FeeStructure            FeeStructureJSON            `json:"feeStructure"`
	SignatureData           SignatureData               `json:"signatureData"`
	OriginalSender          string                      `json:"originalSender"`
}

type CommitmentValidationProof struct {
	A [2]string    `json:"a"`
	B [2][2]string `json:"b"`
	C [2]string    `json:"c"`
}

type CommitmentValidationDataType struct {
	TokenAddresses   []string                  `json:"tokenAddresses"`
	InAmounts        [][]string                `json:"inAmounts"`
	InTimeStamps     [][]string                `json:"inTimeStamps"`
	InRandomizations [][]string                `json:"inRandomizations"`
	InCommitments    [][]string                `json:"inCommitments"`
	InNullifiers     [][]string                `json:"inNullifiers"`
	Proof            CommitmentValidationProof `json:"proof"`
}
