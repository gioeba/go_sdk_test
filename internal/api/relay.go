package api

import (
	"context"
	"fmt"

	"github.com/gioeba/go_sdk_test/constants"
)

type idleRelayResponse struct {
	Relay string `json:"relay"`
}

func GetIdleRelay(ctx context.Context, chainID int, markAsPending bool) (string, error) {
	url := fmt.Sprintf("%s%s?markAsPending=%t&chainId=%d",
		constants.GetRelayerURL(), constants.RelayerConfig.GetIdleRelay, markAsPending, chainID)
	var resp idleRelayResponse
	if err := Get(ctx, url, &resp); err != nil {
		return "", err
	}
	return resp.Relay, nil
}
