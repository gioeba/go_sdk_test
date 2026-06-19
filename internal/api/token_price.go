package api

import (
	"context"

	"github.com/gioeba/go_sdk_test/constants"
)

type GetTokenPricesResponse struct {
	Prices []float64 `json:"prices"`
}

func GetTokenPrices(ctx context.Context, chainID int, erc20Addresses []string) (*GetTokenPricesResponse, error) {
	body := map[string]any{
		"erc20Addresses": erc20Addresses,
		"chainId":        chainID,
	}
	var resp GetTokenPricesResponse
	if err := Post(ctx, constants.GetRelayerURL()+constants.RelayerConfig.GetTokenPrices, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
