package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	tronclient "github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"

	"github.com/gioeba/go_sdk_test/types"
)

type SignableTronClient struct {
	grpcClient *tronclient.GrpcClient
	signer     types.TronSigner
	address    string
}

func newSignableTronClient(grpcClient *tronclient.GrpcClient, signer types.TronSigner, address string) *SignableTronClient {
	return &SignableTronClient{grpcClient: grpcClient, signer: signer, address: address}
}

func (c *SignableTronClient) SignAndBroadcast(ctx context.Context, tx *core.Transaction) (string, error) {
	signed, err := c.signer.SignTransaction(ctx, tx)
	if err != nil {
		return "", fmt.Errorf("sign tron transaction: %w", err)
	}
	result, err := c.grpcClient.Broadcast(signed)
	if err != nil {
		return "", fmt.Errorf("broadcast tron transaction: %w", err)
	}
	if !result.Result {
		return "", fmt.Errorf("broadcast rejected: %s", result.Message)
	}
	rawBytes, err := proto.Marshal(signed.GetRawData())
	if err != nil {
		return "", fmt.Errorf("marshal raw data for txid: %w", err)
	}
	hash := sha256.Sum256(rawBytes)
	return hex.EncodeToString(hash[:]), nil
}

func (c *SignableTronClient) GetAddress() string {
	return c.address
}

func (c *SignableTronClient) GrpcClient() *tronclient.GrpcClient {
	return c.grpcClient
}
