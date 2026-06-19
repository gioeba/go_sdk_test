package api

import (
	"context"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

type UpdateDepositAndWithdrawStatusRequestBody struct {
	ID                    string                        `json:"id,omitempty"`
	HashedEthereumAddress string                        `json:"hashedEthereumAddress"`
	ChainID               int                           `json:"chainId"`
	Phase                 types.DepositAndWithdrawPhase `json:"phase"`
	DepositTxHash         string                        `json:"depositTxHash,omitempty"`
	ScheduleID            string                        `json:"scheduleId,omitempty"`
}

type DepositAndWithdrawStatusResponse struct {
	Status        string                        `json:"status"`
	ID            string                        `json:"id,omitempty"`
	Message       string                        `json:"message,omitempty"`
	Phase         types.DepositAndWithdrawPhase `json:"phase,omitempty"`
	DepositTxHash string                        `json:"depositTxHash,omitempty"`
	ScheduleID    string                        `json:"scheduleId,omitempty"`
	UpdatedAt     string                        `json:"updatedAt,omitempty"`
}

func UpdateDepositAndWithdrawStatus(ctx context.Context, body UpdateDepositAndWithdrawStatusRequestBody) (DepositAndWithdrawStatusResponse, error) {
	var resp DepositAndWithdrawStatusResponse
	if err := Post(ctx, constants.GetRelayerURL()+constants.RelayerConfig.UpdateDepositAndWithdrawStatus, body, &resp); err != nil {
		return DepositAndWithdrawStatusResponse{}, err
	}
	return resp, nil
}

func SafeUpdateDepositAndWithdrawStatus(ctx context.Context, body UpdateDepositAndWithdrawStatusRequestBody) (*DepositAndWithdrawStatusResponse, error) {
	if body.ID == "" {
		return nil, nil
	}
	resp, err := UpdateDepositAndWithdrawStatus(ctx, body)
	if err != nil {
		return nil, nil
	}
	return &resp, nil
}
