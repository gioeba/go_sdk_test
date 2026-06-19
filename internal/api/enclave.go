package api

import (
	"context"
	"encoding/json"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
)

type DecryptUtxoEnclaveResponse struct {
	Utxos            []types.UtxoParams               `json:"utxos"`
	EncryptedOutputs []*types.EncryptedOutputWithSign `json:"encryptedOutputs"`
	LastOutput       string                           `json:"lastOutput"`
}

func DecryptUtxoEnclaveCall(ctx context.Context, chainID int, keyCiphertext, inputCiphertext string) (*DecryptUtxoEnclaveResponse, error) {
	body := map[string]any{
		"chainId": chainID,
		"input":   inputCiphertext,
		"key":     keyCiphertext,
	}
	var resp DecryptUtxoEnclaveResponse
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.DecryptUtxos, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type StoreUtxoResponse struct {
	Status string `json:"status"`
}

func StoreUtxoEnclaveCall(ctx context.Context, recipientEthAddress, encryptedUtxo, key string, chainID int) (*StoreUtxoResponse, error) {
	body := map[string]any{
		"recipientEthAddress": recipientEthAddress,
		"encryptedUtxo":       encryptedUtxo,
		"key":                 key,
		"chainId":             chainID,
	}
	var resp StoreUtxoResponse
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.StoreUtxo, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type GetUtxosResponse struct {
	Utxos []json.RawMessage `json:"utxos"`
}

func GetUtxosEnclaveCall(
	ctx context.Context,
	ethAddress, encryptedSignature, key string,
	chainID int,
	isSolanaLedger bool,
	txMessageForSolanaLedger string,
) (*GetUtxosResponse, error) {
	body := map[string]any{
		"ethAddress":         ethAddress,
		"encryptedSignature": encryptedSignature,
		"key":                key,
		"chainId":            chainID,
		"isSolanaLedger":     isSolanaLedger,
	}
	if txMessageForSolanaLedger != "" {
		body["txMessageForSolanaLedger"] = txMessageForSolanaLedger
	}
	var resp GetUtxosResponse
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.GetUtxos, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type StoreAndGetSignatureResponse struct {
	Signature string `json:"signature"`
}

func StoreAndGetSignatureEnclaveCall(
	ctx context.Context,
	ethAddress, encryptedSignature, key string,
	isSolanaLedger bool,
	txMessageForSolanaLedger string,
) (*StoreAndGetSignatureResponse, error) {
	body := map[string]any{
		"ethAddress":         ethAddress,
		"encryptedSignature": encryptedSignature,
		"key":                key,
		"isSolanaLedger":     isSolanaLedger,
	}
	if txMessageForSolanaLedger != "" {
		body["txMessageForSolanaLedger"] = txMessageForSolanaLedger
	}
	var resp StoreAndGetSignatureResponse
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.StoreAndGetSignature, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type SignProofRequest struct {
	A          []string   `json:"a"`
	B          [][]string `json:"b"`
	C          []string   `json:"c"`
	Inputs     []string   `json:"inputs"`
	VerifierID string     `json:"verifier_id"`
}

type signProofResponse struct {
	Signature string `json:"signature"`
}

func SignProofEnclaveCall(ctx context.Context, req SignProofRequest) (string, error) {
	var resp signProofResponse
	if err := Post(ctx, constants.GetEnclaveURL()+"/sign-proof", req, &resp); err != nil {
		return "", err
	}
	return resp.Signature, nil
}

func GenerateProofsEnclaveCall(
	ctx context.Context,
	wasmFilenames, zkeyFilenames []string,
	inputCiphertext, keyCiphertext string,
) ([]types.GenerateProofResponseType, error) {
	body := map[string]any{
		"circuit_wasms": wasmFilenames,
		"circuit_zkeys": zkeyFilenames,
		"inputs":        inputCiphertext,
		"key":           keyCiphertext,
	}
	var resp []types.GenerateProofResponseType
	if err := Post(ctx, constants.GetEnclaveURL()+constants.EnclaveConfig.GenerateProofs, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
