// Package hinkal is the public entry point for the Hinkal Protocol Go SDK.
package hinkal

import (
	core "github.com/gioeba/go_sdk_test/data-structures/hinkal"
	"github.com/gioeba/go_sdk_test/providers"
	"github.com/gioeba/go_sdk_test/signers"
	"github.com/gioeba/go_sdk_test/types"
)

type Hinkal = core.Hinkal

type Config = types.HinkalConfig

func New(cfg *Config) *Hinkal { return core.NewHinkal(cfg) }

type LoginMessageMode = types.LoginMessageMode

const (
	LoginMessageModeProtocol        = types.LoginMessageModeProtocol
	LoginMessageModePrivateTransfer = types.LoginMessageModePrivateTransfer
)

var (
	NewPrivateKeyEVMSigner    = signers.NewPrivateKeyEVMSigner
	NewPrivateKeyTronSigner   = signers.NewPrivateKeyTronSigner
	NewPrivateKeySolanaSigner = signers.NewPrivateKeySolanaSigner
)

var (
	NewEthersProviderAdapter = providers.NewEthersProviderAdapter
	NewTronProviderAdapter   = providers.NewTronProviderAdapter
	NewSolanaProviderAdapter = providers.NewSolanaProviderAdapter
)
