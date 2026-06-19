package snarkjs

import (
	"math/big"

	"github.com/gioeba/go_sdk_test/types"
)

func bigIntPtrString(n *big.Int) *string {
	if n == nil {
		return nil
	}
	s := n.String()
	return &s
}

func bigIntSliceStrings(values []*big.Int) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v.String()
	}
	return out
}

func SerializeCircomData(c types.CircomDataType) types.CircomDataJSONType {
	return types.CircomDataJSONType{
		RootHashHinkal:      bigIntPtrString(c.RootHashHinkal),
		Erc20TokenAddresses: c.Erc20TokenAddresses,
		TokenIDs:            c.TokenIDs,
		AmountChanges:       bigIntSliceStrings(c.AmountChanges),
		InputNullifiers:     c.InputNullifiers,
		OutCommitments:      c.OutCommitments,
		EncryptedOutputs:    c.EncryptedOutputs,
		StealthAddressStructure: types.StealthAddressStructureJSON{
			ExtraRandomization: c.StealthAddressStructure.ExtraRandomization.String(),
			StealthAddress:     c.StealthAddressStructure.StealthAddress.String(),
			H0:                 c.StealthAddressStructure.H0.String(),
			H1:                 c.StealthAddressStructure.H1.String(),
		},
		TimeStamp:              c.TimeStamp,
		RootHashAccessToken:    bigIntPtrString(c.RootHashAccessToken),
		Relay:                  c.Relay,
		ExternalAddress:        c.ExternalAddress,
		ExternalActionMetadata: c.ExternalActionMetadata,
		ExternalActionID:       c.ExternalActionID.String(),
		HookData:               c.HookData,
		CalldataHash:           c.CalldataHash.String(),
		PublicSignalCount:      c.PublicSignalCount,
		OnChainCreation:        c.OnChainCreation,
		SlippageValues:         bigIntSliceStrings(c.SlippageValues),
		HinkalLogicArgs:        serializeHinkalLogicArgs(c.HinkalLogicArgs),
		FeeStructure: types.FeeStructureJSON{
			FeeToken:     c.FeeStructure.FeeToken,
			FlatFee:      c.FeeStructure.FlatFee.String(),
			VariableRate: c.FeeStructure.VariableRate.String(),
		},
		SignatureData:  c.SignatureData,
		OriginalSender: c.OriginalSender,
	}
}

func serializeHinkalLogicArgs(h types.HinkalLogicArgs) types.HinkalLogicArgsJSON {
	useApprovalUtxoData := make([]types.UseApprovalUTXODataJSON, len(h.UseApprovalUtxoData))
	for i, ob := range h.UseApprovalUtxoData {
		useApprovalUtxoData[i] = types.UseApprovalUTXODataJSON{
			ApprovalChanges:           bigIntSliceStrings(ob.ApprovalChanges),
			ExternalApprovalAddresses: ob.ExternalApprovalAddresses,
			ConversionInHinkalAddress: bigIntSliceStrings(ob.ConversionInHinkalAddress),
		}
	}
	return types.HinkalLogicArgsJSON{
		HinkalLogicAction:      h.HinkalLogicAction,
		ExecuteApprovalChanges: bigIntSliceStrings(h.ExecuteApprovalChanges),
		DoPreTxApproval:        h.DoPreTxApproval,
		InHinkalAddress:        h.InHinkalAddress.String(),
		UseApprovalUtxoData:    useApprovalUtxoData,
		InteractionAddress:     h.InteractionAddress,
	}
}
