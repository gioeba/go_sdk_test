package tron

import (
	"context"
	"fmt"
	"math/big"

	tronclient "github.com/fbsobreira/gotron-sdk/pkg/client"
	gotronapi "github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"

	"github.com/gioeba/go_sdk_test/constants"
)

func availableResource(limit, used int64) *big.Int {
	if limit > used {
		return big.NewInt(limit - used)
	}
	return big.NewInt(0)
}

func EstimateTronFeeSunWithPadding(
	ctx context.Context,
	grpc *tronclient.GrpcClient,
	ownerBase58, contractBase58 string,
	callData []byte,
	callValueSun *big.Int,
) (*big.Int, error) {
	callValue := int64(0)
	if callValueSun != nil && callValueSun.Sign() > 0 {
		callValue = callValueSun.Int64()
	}

	constantResult, err := grpc.TriggerConstantContractWithDataCtx(ctx, ownerBase58, contractBase58, callData, tronclient.WithCallValue(callValue))
	if err != nil {
		return nil, fmt.Errorf("tron constant contract call: %w", err)
	}

	usedEnergy := big.NewInt(constantResult.GetEnergyUsed())
	if estimateResult, err := grpc.EstimateEnergyWithDataCtx(ctx, ownerBase58, contractBase58, callData, callValue, "", 0); err == nil {
		if required := estimateResult.GetEnergyRequired(); required > 0 {
			usedEnergy = big.NewInt(required)
		}
	}

	chainParams, err := grpc.Client.GetChainParameters(ctx, new(gotronapi.EmptyMessage))
	if err != nil {
		return nil, fmt.Errorf("tron chain parameters: %w", err)
	}
	energyUnitPriceSun, err := getChainParameterValue(chainParams, "getEnergyFee")
	if err != nil {
		return nil, err
	}
	bandwidthUnitPriceSun, err := getChainParameterValue(chainParams, "getTransactionFee")
	if err != nil {
		return nil, err
	}

	resources, err := grpc.GetAccountResourceCtx(ctx, ownerBase58)
	if err != nil {
		return nil, fmt.Errorf("tron account resources: %w", err)
	}

	availableEnergy := availableResource(resources.GetEnergyLimit(), resources.GetEnergyUsed())
	payableEnergy := new(big.Int).Sub(usedEnergy, availableEnergy)
	if payableEnergy.Sign() < 0 {
		payableEnergy = big.NewInt(0)
	}
	payableEnergySun := new(big.Int).Mul(payableEnergy, energyUnitPriceSun)

	rawData, err := proto.Marshal(constantResult.GetTransaction().GetRawData())
	if err != nil {
		return nil, fmt.Errorf("tron marshal raw data: %w", err)
	}
	txSizeBytes := big.NewInt(int64(len(rawData)))

	freeNetLeft := availableResource(resources.GetFreeNetLimit(), resources.GetFreeNetUsed())
	stakedNetLeft := availableResource(resources.GetNetLimit(), resources.GetNetUsed())
	availableBandwidth := new(big.Int).Add(freeNetLeft, stakedNetLeft)
	payableBandwidth := new(big.Int).Sub(txSizeBytes, availableBandwidth)
	if payableBandwidth.Sign() < 0 {
		payableBandwidth = big.NewInt(0)
	}
	payableBandwidthSun := new(big.Int).Mul(payableBandwidth, bandwidthUnitPriceSun)

	estimatedFeeSun := new(big.Int).Add(payableEnergySun, payableBandwidthSun)
	padding := new(big.Int).Mul(estimatedFeeSun, big.NewInt(constants.TronFeePaddingBps))
	padding.Div(padding, big.NewInt(10_000))
	paddedFeeSun := new(big.Int).Add(estimatedFeeSun, padding)

	feeLimit := big.NewInt(constants.TronDefaultFeeLimitSun)
	if paddedFeeSun.Cmp(feeLimit) > 0 {
		return feeLimit, nil
	}
	return paddedFeeSun, nil
}

func getChainParameterValue(params *core.ChainParameters, key string) (*big.Int, error) {
	for _, p := range params.GetChainParameter() {
		if p.GetKey() == key {
			return big.NewInt(p.GetValue()), nil
		}
	}
	return nil, fmt.Errorf("missing Tron chain parameter: %s", key)
}
