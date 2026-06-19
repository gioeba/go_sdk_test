package api

import (
	"context"

	"github.com/gioeba/go_sdk_test/constants"
)

type TemporaryWalletNoncesResponse struct {
	Nonces []int `json:"nonces"`
}

type AddTemporaryWalletNonceRequest struct {
	HashedEthereumAddress string `json:"hashedEthereumAddress"`
	ChainID               int    `json:"chainId"`
	Nonce                 int    `json:"nonce"`
}

type TemporaryWalletNonceResponse struct {
	Success bool `json:"success"`
}

func GetTemporaryWalletNonces(ctx context.Context, chainID int, hashedEthereumAddress string) (TemporaryWalletNoncesResponse, error) {
	var resp TemporaryWalletNoncesResponse
	url := constants.GetServerURL() + constants.ServerConfig.GetTemporaryWalletNonces(hashedEthereumAddress, chainID)
	if err := Get(ctx, url, &resp); err != nil {
		return TemporaryWalletNoncesResponse{}, err
	}
	return resp, nil
}

func AddTemporaryWalletNonce(ctx context.Context, chainID int, hashedEthereumAddress string, nonce int) (TemporaryWalletNonceResponse, error) {
	var resp TemporaryWalletNonceResponse
	url := constants.GetServerURL() + constants.ServerConfig.AddTemporaryWalletNonce
	if err := Post(ctx, url, AddTemporaryWalletNonceRequest{
		HashedEthereumAddress: hashedEthereumAddress,
		ChainID:               chainID,
		Nonce:                 nonce,
	}, &resp); err != nil {
		return TemporaryWalletNonceResponse{}, err
	}
	return resp, nil
}
