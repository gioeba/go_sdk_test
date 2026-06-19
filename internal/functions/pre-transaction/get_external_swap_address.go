package pretransaction

import (
	"errors"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

var errNoExternalSwapAddress = errors.New("pretransaction: no external address set during swap")

func GetExternalSwapAddress(chainID int, externalActionID types.ExternalActionID) (string, error) {
	contractData, err := constants.GetContractData(chainID)
	if err != nil {
		return "", err
	}

	var externalAddress string
	switch externalActionID {
	case types.ExternalActionUniswap:
		externalAddress = contractData.UniswapExternalActionAddress
	case types.ExternalActionOdos:
		externalAddress = contractData.OdosExternalActionInstanceAddress
	case types.ExternalActionOneInch:
		externalAddress = contractData.OneInchExternalActionInstanceAddress
	case types.ExternalActionLifi:
		externalAddress = contractData.HinkalAddress
	default:
		externalAddress = contractData.UniswapExternalActionAddress
	}
	if externalAddress == "" {
		return "", errNoExternalSwapAddress
	}
	return externalAddress, nil
}
