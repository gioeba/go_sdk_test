package privatewallet

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"

	"github.com/gioeba/go_sdk_test/constants"
	sdktypes "github.com/gioeba/go_sdk_test/types"
)

const eip7702Magic = "ef0100"

const emporiumERC20ABIJSON = `[
  {"name":"transfer","type":"function","stateMutability":"nonpayable","inputs":[{"name":"recipient","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]},
  {"name":"approve","type":"function","stateMutability":"nonpayable","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]}
]`

var emporiumERC20ABI = func() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(emporiumERC20ABIJSON))
	if err != nil {
		panic(fmt.Sprintf("privatewallet: invalid erc20 abi: %v", err))
	}
	return parsed
}()

func EmporiumOp(contract string, callDataString string, invokeWallet bool, value *big.Int) (string, error) {
	endpoint := common.FromHex(contract)
	if len(endpoint) != 20 {
		return "", fmt.Errorf("privatewallet: invalid emporium endpoint %s", contract)
	}
	if value == nil {
		value = big.NewInt(0)
	}
	if value.Sign() < 0 || value.BitLen() > 128 {
		return "", errors.New("privatewallet: emporium op value must fit uint128")
	}

	invokeWalletByte := byte(0)
	if invokeWallet {
		invokeWalletByte = 1
	}
	valueBytes := value.FillBytes(make([]byte, 16))
	callData := common.FromHex(callDataString)

	op := make([]byte, 0, 20+1+16+len(callData))
	op = append(op, endpoint...)
	op = append(op, invokeWalletByte)
	op = append(op, valueBytes...)
	op = append(op, callData...)
	return "0x" + hex.EncodeToString(op), nil
}

func CreateTransferToEmporiumOp(tokenAddress string, to string, amount *big.Int, invokeWallet bool) (string, error) {
	if strings.EqualFold(tokenAddress, constants.ZeroAddress) {
		return EmporiumOp(to, "", invokeWallet, amount)
	}
	data, err := emporiumERC20ABI.Pack("transfer", common.HexToAddress(to), amount)
	if err != nil {
		return "", err
	}
	return EmporiumOp(tokenAddress, "0x"+hex.EncodeToString(data), invokeWallet, nil)
}

func CreateApproveEmporiumOp(tokenAddress string, to string, amount *big.Int, invokeWallet bool) (string, error) {
	data, err := emporiumERC20ABI.Pack("approve", common.HexToAddress(to), amount)
	if err != nil {
		return "", err
	}
	return EmporiumOp(tokenAddress, "0x"+hex.EncodeToString(data), invokeWallet, nil)
}

func GenerateFundAndApproveOps(
	erc20AddressesToFund []string,
	fundAmounts []*big.Int,
	approveTokenAddresses []string,
	approveAmounts []*big.Int,
	walletAddress string,
	spenderAddress string,
) ([]string, error) {
	if len(erc20AddressesToFund) != len(fundAmounts) {
		return nil, errors.New("privatewallet: fund tokens and amounts length mismatch")
	}
	if len(approveTokenAddresses) != len(approveAmounts) {
		return nil, errors.New("privatewallet: approve tokens and amounts length mismatch")
	}

	ops := make([]string, 0, len(erc20AddressesToFund)+len(approveTokenAddresses))
	for i, tokenAddress := range erc20AddressesToFund {
		op, err := CreateTransferToEmporiumOp(tokenAddress, walletAddress, fundAmounts[i], false)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	if spenderAddress != "" {
		for i, tokenAddress := range approveTokenAddresses {
			if strings.EqualFold(tokenAddress, constants.ZeroAddress) {
				continue
			}
			op, err := CreateApproveEmporiumOp(tokenAddress, spenderAddress, approveAmounts[i], true)
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}
	}
	return ops, nil
}

func CreateLifiBridgeOps(
	chainID int,
	fromAddress string,
	lifiRouterAddress string,
	tokenAddress string,
	fundAmount *big.Int,
	bridgeAmount *big.Int,
	quote sdktypes.BridgeQuote,
) ([]string, error) {
	_ = chainID
	tokensToFund := []string{tokenAddress}
	fundAmounts := []*big.Int{fundAmount}

	nativeFee := quote.NativeFee
	if nativeFee == nil {
		nativeFee = big.NewInt(0)
	}
	if nativeFee.Sign() > 0 && !strings.EqualFold(tokenAddress, constants.ZeroAddress) {
		tokensToFund = append(tokensToFund, constants.ZeroAddress)
		fundAmounts = append(fundAmounts, nativeFee)
	}

	ops, err := GenerateFundAndApproveOps(
		tokensToFund,
		fundAmounts,
		[]string{tokenAddress},
		[]*big.Int{bridgeAmount},
		fromAddress,
		lifiRouterAddress,
	)
	if err != nil {
		return nil, err
	}

	nativeValue := new(big.Int).Set(nativeFee)
	if strings.EqualFold(tokenAddress, constants.ZeroAddress) {
		nativeValue.Add(nativeValue, bridgeAmount)
	}
	if nativeValue.Sign() == 0 {
		nativeValue = nil
	}
	bridgeOp, err := EmporiumOp(lifiRouterAddress, quote.Calldata, true, nativeValue)
	if err != nil {
		return nil, err
	}
	return append(ops, bridgeOp), nil
}

func ConvertEmporiumOpToCallInfo(op string, walletAddress string, chainID int) (sdktypes.CallInfo, error) {
	opBytes := common.FromHex(op)
	if len(opBytes) < 37 {
		return sdktypes.CallInfo{}, errors.New("privatewallet: invalid emporium op")
	}
	contractData, err := constants.GetContractData(chainID)
	if err != nil {
		return sdktypes.CallInfo{}, err
	}

	to := common.BytesToAddress(opBytes[:20]).Hex()
	invokeWallet := opBytes[20] > 0
	value := new(big.Int).SetBytes(opBytes[21:37])
	calldata := "0x" + hex.EncodeToString(opBytes[37:])
	from := contractData.HinkalAddress
	if invokeWallet {
		from = walletAddress
	}
	return sdktypes.CallInfo{
		From:     from,
		To:       to,
		Calldata: calldata,
		Value:    value,
	}, nil
}

func GetAuthorizationDataIfNeeded(ctx context.Context, client *ethclient.Client, chainID int, privateKey string) (*sdktypes.AuthorizationData, error) {
	if chainID == constants.ChainIDs.Localhost || privateKey == "" {
		return nil, nil
	}
	key, err := gethcrypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		return nil, fmt.Errorf("privatewallet: authorization key: %w", err)
	}
	eoaAddress := gethcrypto.PubkeyToAddress(key.PublicKey)
	contractData, err := constants.GetContractData(chainID)
	if err != nil {
		return nil, err
	}
	if contractData.HinkalWalletAddress == "" || strings.EqualFold(contractData.HinkalWalletAddress, constants.ZeroAddress) {
		return nil, errors.New("privatewallet: Hinkal wallet address is not set")
	}
	implementation := common.HexToAddress(contractData.HinkalWalletAddress)

	code, err := client.CodeAt(ctx, eoaAddress, nil)
	if err != nil {
		return nil, fmt.Errorf("privatewallet: get temporary wallet code: %w", err)
	}
	if len(code) > 0 {
		if len(code) != 23 || !bytes.Equal(code[:3], common.FromHex(eip7702Magic)) {
			return nil, errors.New("privatewallet: temporary wallet address already has non-EIP-7702 code")
		}
		if bytes.Equal(code[3:], implementation.Bytes()) {
			return nil, nil
		}
	}

	nonce, err := client.NonceAt(ctx, eoaAddress, nil)
	if err != nil {
		return nil, fmt.Errorf("privatewallet: get temporary wallet nonce: %w", err)
	}
	auth, err := types.SignSetCode(key, types.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(big.NewInt(int64(chainID))),
		Address: implementation,
		Nonce:   nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("privatewallet: sign authorization: %w", err)
	}

	r := auth.R.ToBig().FillBytes(make([]byte, 32))
	s := auth.S.ToBig().FillBytes(make([]byte, 32))
	return &sdktypes.AuthorizationData{
		V:       fmt.Sprintf("%d", auth.V+27),
		R:       "0x" + hex.EncodeToString(r),
		S:       "0x" + hex.EncodeToString(s),
		Nonce:   fmt.Sprintf("%d", auth.Nonce),
		Address: implementation.Hex(),
		ChainID: fmt.Sprintf("%d", chainID),
	}, nil
}
