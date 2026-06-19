package signers

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	tronaddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	troncommon "github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	proto "google.golang.org/protobuf/proto"
)

type PrivateKeyTronSigner struct {
	key     *ecdsa.PrivateKey
	address tronaddress.Address
}

func NewPrivateKeyTronSigner(privateKeyHex string) (*PrivateKeyTronSigner, error) {
	hex := strings.TrimPrefix(privateKeyHex, "0x")
	key, err := crypto.HexToECDSA(hex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	return &PrivateKeyTronSigner{
		key:     key,
		address: tronaddress.PubkeyToAddress(key.PublicKey),
	}, nil
}

func (s *PrivateKeyTronSigner) GetAddress(_ context.Context) (string, error) {
	return s.address.String(), nil
}

func (s *PrivateKeyTronSigner) SignMessage(_ context.Context, message string) ([]byte, error) {
	data := []byte(message)
	msg := fmt.Sprintf("\x19TRON Signed Message:\n%d%s", len(data), message)
	hash := troncommon.Keccak256([]byte(msg))
	sig, err := crypto.Sign(hash, s.key)
	if err != nil {
		return nil, fmt.Errorf("sign message: %w", err)
	}
	// Tron personal_sign uses same 27/28 convention as Ethereum for message signatures.
	sig[64] += 27
	return sig, nil
}

func (s *PrivateKeyTronSigner) SignTransaction(_ context.Context, tx *core.Transaction) (*core.Transaction, error) {
	rawData, err := proto.Marshal(tx.GetRawData())
	if err != nil {
		return nil, fmt.Errorf("marshal tx raw data: %w", err)
	}
	h := sha256.Sum256(rawData)
	sig, err := crypto.Sign(h[:], s.key)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	tx.Signature = append(tx.Signature, sig)
	return tx, nil
}
