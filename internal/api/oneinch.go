package api

import (
	"context"
	"fmt"
	"math/big"
	"net/url"

	"github.com/gioeba/go_sdk_test/constants"
)

type OneInchRequest struct {
	FromTokenAddress string
	ToTokenAddress   string
	Amount           *big.Int
	FromAddress      string
	Slippage         float64
	DestReceiver     string
	DisableEstimate  bool
	AllowPartialFill bool
}

type OneInchResponse struct {
	Tx struct {
		Data string `json:"data"`
	} `json:"tx"`
	ToAmount string `json:"toAmount"`
}

type callOneInchAPIResponse struct {
	OneInchResponse OneInchResponse `json:"oneInchResponse"`
}

func CallOneInchAPI(ctx context.Context, chainID int, request OneInchRequest) (OneInchResponse, error) {
	params := url.Values{}
	params.Set("fromTokenAddress", request.FromTokenAddress)
	params.Set("toTokenAddress", request.ToTokenAddress)
	params.Set("amount", request.Amount.String())
	params.Set("fromAddress", request.FromAddress)
	params.Set("slippage", fmt.Sprintf("%v", request.Slippage))
	if request.DestReceiver != "" {
		params.Set("destReceiver", request.DestReceiver)
	}
	if request.DisableEstimate {
		params.Set("disableEstimate", "true")
	}
	if request.AllowPartialFill {
		params.Set("allowPartialFill", "true")
	}

	body := map[string]any{
		"url": fmt.Sprintf("https://api.1inch.dev/swap/v5.2/%d/swap?%s", chainID, params.Encode()),
	}
	var resp callOneInchAPIResponse
	apiURL := constants.GetServerURL() + constants.ServerConfig.CallOneInchAPI
	if err := Post(ctx, apiURL, body, &resp); err != nil {
		return OneInchResponse{}, err
	}
	return resp.OneInchResponse, nil
}
