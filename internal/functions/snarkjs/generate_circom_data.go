package snarkjs

import (
	"math/big"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

func GenerateCircomData(
	outCommitments [][]string,
	inputNullifiers [][]string,
	rootHashHinkal *big.Int,
	amountChanges []*big.Int,
	erc20TokenAddresses []string,
	outputUtxos [][]*utxo.Utxo,
	encryptedOutputs [][]string,
	publicSignalCount int,
	externalActionID types.ExternalActionID,
	externalAddress string,
	externalActionMetadata string,
	relay string,
	calldataHash *big.Int,
	stealthAddressStructure types.StealthAddressStructure,
	onChainCreation []bool,
	hookData *types.HookDataType,
	timeStampFallback *string,
	slippageValues []*big.Int,
	feeStructure types.FeeStructure,
	signatureData types.SignatureData,
	originalSender string,
) types.CircomDataType {
	tokenIDs := make([]string, len(erc20TokenAddresses))
	for i := range tokenIDs {
		tokenIDs[i] = "0"
	}

	timeStamp := ""
	if len(outputUtxos) > 0 {
		timeStamp = outputUtxos[0][0].TimeStamp
	} else if timeStampFallback != nil {
		timeStamp = *timeStampFallback
	}

	hd := types.DefaultHookData()
	if hookData != nil {
		hd = *hookData
	}

	externalAddressOrZero := externalAddress
	if externalAddressOrZero == "" {
		externalAddressOrZero = constants.ZeroAddress
	}

	sender := originalSender
	if sender == "" {
		sender = GetOriginalSender(externalAddressOrZero, relay)
	}

	return types.CircomDataType{
		RootHashHinkal:          rootHashHinkal,
		Erc20TokenAddresses:     erc20TokenAddresses,
		TokenIDs:                tokenIDs,
		AmountChanges:           amountChanges,
		InputNullifiers:         inputNullifiers,
		OutCommitments:          outCommitments,
		EncryptedOutputs:        encryptedOutputs,
		TimeStamp:               timeStamp,
		StealthAddressStructure: stealthAddressStructure,
		RootHashAccessToken:     big.NewInt(0),
		Relay:                   relay,
		ExternalAddress:         externalAddressOrZero,
		ExternalActionMetadata:  externalActionMetadata,
		ExternalActionID:        GetExternalActionIDHash(externalActionID),
		HookData:                hd,
		PublicSignalCount:       publicSignalCount,
		CalldataHash:            calldataHash,
		OnChainCreation:         onChainCreation,
		SlippageValues:          slippageValues,
		HinkalLogicArgs:         types.DefaultHinkalLogicArgs(len(erc20TokenAddresses)),
		FeeStructure:            feeStructure,
		SignatureData:           signatureData,
		OriginalSender:          sender,
	}
}
