package api

import (
	"context"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

func GetNearIntentsQuote(ctx context.Context, req types.NearIntentsQuoteRequest) (types.NearIntentsQuoteResponse, error) {
	var resp types.NearIntentsQuoteResponse
	url := constants.GetServerURL() + constants.ServerConfig.CallNearIntentsQuote
	if err := Post(ctx, url, req, &resp); err != nil {
		return types.NearIntentsQuoteResponse{}, err
	}
	return resp, nil
}

func GetNearIntentsTokens(ctx context.Context) ([]types.NearIntentsToken, error) {
	var resp []types.NearIntentsToken
	url := constants.GetServerURL() + constants.ServerConfig.CallNearIntentsTokens
	if err := Get(ctx, url, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
