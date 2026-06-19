package api

import (
	"context"
	"encoding/json"

	"github.com/gioeba/go_sdk_test/constants"
)

type OdosInputToken struct {
	TokenAddress string `json:"tokenAddress"`
	Amount       string `json:"amount"`
}

type OdosOutputToken struct {
	TokenAddress string `json:"tokenAddress"`
	Proportion   int    `json:"proportion"`
}

type OdosSwapRequest struct {
	ChainID              int               `json:"chainId"`
	InputTokens          []OdosInputToken  `json:"inputTokens"`
	OutputTokens         []OdosOutputToken `json:"outputTokens"`
	UserAddr             string            `json:"userAddr"`
	SlippageLimitPercent float64           `json:"slippageLimitPercent,omitempty"`
	DisableRFQs          bool              `json:"disableRFQs,omitempty"`
	ReferralCode         string            `json:"referralCode,omitempty"`
}

type OdosSwapDataOutputToken struct {
	TokenAddress string      `json:"tokenAddress"`
	Amount       json.Number `json:"amount"`
}

type OdosSwapData struct {
	OutputTokens []OdosSwapDataOutputToken `json:"outputTokens"`
	Transaction  struct {
		Data string `json:"data"`
	} `json:"transaction"`
}

type callOdosAPIResponse struct {
	OdosResponse OdosSwapData `json:"odosResponse"`
	Status       string       `json:"status"`
	Message      string       `json:"message"`
}

func CallOdosAPI(ctx context.Context, chainID int, request OdosSwapRequest) (OdosSwapData, string, error) {
	if chainID != constants.ChainIDs.Base && chainID != constants.ChainIDs.Localhost {
		request.ReferralCode = "702077826"
	}
	var resp callOdosAPIResponse
	url := constants.GetServerURL() + constants.ServerConfig.CallOdosAPI
	if err := Post(ctx, url, request, &resp); err != nil {
		return OdosSwapData{}, "", err
	}
	return resp.OdosResponse, resp.Status, nil
}
