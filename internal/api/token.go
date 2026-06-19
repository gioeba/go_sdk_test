package api

import (
	"context"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

type getTokensInfoRequest struct {
	ChainID             int      `json:"chainId"`
	TokenAddresses      []string `json:"tokenAddresses"`
	WaitForOffsetUpdate bool     `json:"waitForOffsetUpdate"`
}

func GetTokensInfo(ctx context.Context, chainID int, tokenAddresses []string, waitForOffsetUpdate bool) ([]*types.ERC20Token, error) {
	if len(tokenAddresses) == 0 {
		return nil, nil
	}
	var resp []*types.ERC20Token
	body := getTokensInfoRequest{
		ChainID:             chainID,
		TokenAddresses:      tokenAddresses,
		WaitForOffsetUpdate: waitForOffsetUpdate,
	}
	if err := Post(ctx, constants.GetServerURL()+constants.ServerConfig.GetTokensInfo, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
