package types

type SignatureData struct {
	R               string `json:"r"`
	S               string `json:"s"`
	V               string `json:"v"`
	AccessKey       string `json:"accessKey"`
	Nonce           int    `json:"nonce"`
	EthereumAddress string `json:"ethereumAddress"`
}

const zeroIn32Bytes = "0x0000000000000000000000000000000000000000000000000000000000000000"

func DefaultSignatureData() SignatureData {
	return SignatureData{
		R:               zeroIn32Bytes,
		S:               zeroIn32Bytes,
		V:               zeroIn32Bytes,
		AccessKey:       zeroIn32Bytes,
		Nonce:           0,
		EthereumAddress: zeroAddress,
	}
}
