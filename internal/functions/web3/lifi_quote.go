package web3

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/types"
)

func parseRequiredLifiBigInt(field string, value api.StringOrNumber) (*big.Int, error) {
	raw := strings.TrimSpace(value.String())
	if raw == "" {
		return nil, fmt.Errorf("web3: Lifi quote missing %s", field)
	}
	parsed, err := utils.ParseBigInt(raw)
	if err != nil {
		return nil, fmt.Errorf("web3: invalid Lifi quote %s %q", field, raw)
	}
	return parsed, nil
}

func parseOptionalLifiBigInt(field string, value api.StringOrNumber) (*big.Int, error) {
	raw := strings.TrimSpace(value.String())
	if raw == "" {
		return big.NewInt(0), nil
	}
	parsed, err := utils.ParseBigInt(raw)
	if err != nil {
		return nil, fmt.Errorf("web3: invalid Lifi quote %s %q", field, raw)
	}
	return parsed, nil
}

func GetLifiPrice(
	ctx context.Context,
	inSwapToken, outSwapToken types.ERC20Token,
	inSwapAmount string,
	bridgingSlippage float64,
	fromAddress string,
	toAddress string,
) (types.BridgeQuote, error) {
	inSwapAmountWei, err := GetAmountInWei(inSwapToken, inSwapAmount)
	if err != nil {
		return types.BridgeQuote{}, err
	}

	toTokenAddress := outSwapToken.Erc20TokenAddress
	requestToAddress := toAddress
	if constants.IsTronLike(outSwapToken.ChainID) {
		toTokenAddress, err = utils.ToTronBase58IfHex(toTokenAddress)
		if err != nil {
			return types.BridgeQuote{}, err
		}
		if requestToAddress != "" {
			requestToAddress, err = utils.ToTronBase58IfHex(requestToAddress)
			if err != nil {
				return types.BridgeQuote{}, err
			}
		}
	}

	request := api.LifiRequestData{
		FromChain:   constants.GetLifiChainID(inSwapToken.ChainID),
		ToChain:     constants.GetLifiChainID(outSwapToken.ChainID),
		FromToken:   inSwapToken.Erc20TokenAddress,
		ToToken:     toTokenAddress,
		FromAddress: fromAddress,
		ToAddress:   requestToAddress,
		FromAmount:  inSwapAmountWei.String(),
		Order:       "FASTEST",
		Slippage:    bridgingSlippage,
	}

	lifiResponse, status, err := api.CallLifiAPI(ctx, request)
	if err != nil {
		return types.BridgeQuote{}, err
	}
	if lifiResponse.TransactionRequest.Data == "" {
		return types.BridgeQuote{}, fmt.Errorf("web3: Lifi quote missing transactionRequest.data (status=%q, tool=%q, type=%q)", status, lifiResponse.Tool, lifiResponse.Type)
	}

	expectedAmount, err := parseRequiredLifiBigInt("estimate.toAmount", lifiResponse.Estimate.ToAmount)
	if err != nil {
		return types.BridgeQuote{}, err
	}
	nativeValue, err := parseOptionalLifiBigInt("transactionRequest.value", lifiResponse.TransactionRequest.Value)
	if err != nil {
		return types.BridgeQuote{}, err
	}
	nativeFee := new(big.Int).Set(nativeValue)
	if constants.IsSolanaLike(inSwapToken.ChainID) {
		nativeFee = big.NewInt(0)
	} else if strings.EqualFold(inSwapToken.Erc20TokenAddress, constants.ZeroAddress) {
		nativeFee.Sub(nativeFee, inSwapAmountWei)
	}

	return types.BridgeQuote{
		Calldata:       lifiResponse.TransactionRequest.Data,
		ExpectedAmount: expectedAmount,
		NativeFee:      nativeFee,
	}, nil
}
