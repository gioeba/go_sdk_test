package contractabi

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/gioeba/go_sdk_test/types"
)

const hinkalWrapperProoflessDepositABIJSON = `[
  {
    "type": "function",
    "name": "prooflessDeposit",
    "stateMutability": "payable",
    "inputs": [
      {"name":"feeRecipient","type":"address"},
      {"name":"feeToken","type":"address"},
      {"name":"feeAmount","type":"uint256"},
      {"name":"erc20Addresses","type":"address[]"},
      {"name":"amounts","type":"uint256[]"},
      {"name":"tokenIds","type":"uint256[]"},
      {
        "name":"stealthAddressStructures",
        "type":"tuple[]",
        "components":[
          {"name":"extraRandomization","type":"uint256"},
          {"name":"stealthAddress","type":"uint256"},
          {"name":"H0","type":"uint256"},
          {"name":"H1","type":"uint256"}
        ]
      }
    ],
    "outputs": []
  }
]`

var hinkalWrapperProoflessDepositABI = func() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(hinkalWrapperProoflessDepositABIJSON))
	if err != nil {
		panic(err)
	}
	return parsed
}()

func toABIStealthAddressStructures(structures []types.StealthAddressStructure) []abiStealthAddressStructure {
	out := make([]abiStealthAddressStructure, len(structures))
	for i, s := range structures {
		out[i] = abiStealthAddressStructure{
			ExtraRandomization: zeroIfNil(s.ExtraRandomization),
			StealthAddress:     zeroIfNil(s.StealthAddress),
			H0:                 zeroIfNil(s.H0),
			H1:                 zeroIfNil(s.H1),
		}
	}
	return out
}

func PackProoflessDeposit(
	chainID int,
	erc20Addresses []string,
	amounts []*big.Int,
	tokenIDs []*big.Int,
	structures []types.StealthAddressStructure,
) ([]byte, error) {
	hinkalABI, err := Hinkal(chainID)
	if err != nil {
		return nil, err
	}
	return hinkalABI.Pack(
		"prooflessDeposit",
		toAddresses(erc20Addresses),
		amounts,
		tokenIDs,
		toABIStealthAddressStructures(structures),
	)
}

func PackProoflessDepositWithPublicFee(
	feeRecipient string,
	feeToken string,
	feeAmount *big.Int,
	erc20Addresses []string,
	amounts []*big.Int,
	tokenIDs []*big.Int,
	structures []types.StealthAddressStructure,
) ([]byte, error) {
	return hinkalWrapperProoflessDepositABI.Pack(
		"prooflessDeposit",
		common.HexToAddress(feeRecipient),
		common.HexToAddress(feeToken),
		zeroIfNil(feeAmount),
		toAddresses(erc20Addresses),
		amounts,
		tokenIDs,
		toABIStealthAddressStructures(structures),
	)
}
