package tests

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/nacl/box"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/crypto"
	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/internal/data-structures/eventservice"
	"github.com/gioeba/go_sdk_test/internal/functions/balance"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/types"
)

func ensure0x(s string) string {
	if strings.HasPrefix(s, "0x") {
		return s
	}
	return "0x" + s
}

func buildUtxoPlaintext(amount *big.Int, token, stealth string, ts *big.Int) []byte {
	var b strings.Builder
	b.WriteString(utils.ToBeHex(amount))
	b.WriteString(ensure0x(token))
	b.WriteString(utils.ToBeHex(big.NewInt(99)))
	b.WriteString(ensure0x(stealth))
	b.WriteString(utils.ToBeHex(ts))
	b.WriteString(utils.ToBeHex(big.NewInt(1)))
	b.WriteString(utils.ToBeHex(big.NewInt(7)))
	b.WriteString(utils.ToBeHex(big.NewInt(9)))
	return []byte(b.String())
}

func sampleUtxoFields() (amount *big.Int, token, stealth string, ts *big.Int) {
	return big.NewInt(1_000_000),
		"0x" + strings.Repeat("11", 20),
		"0x" + strings.Repeat("2a", 31),
		big.NewInt(1_700_000_000)
}

func TestDecryptUtxo_SealedKeysRoundTrip(t *testing.T) {
	uk := cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	_, pk, err := cryptokeys.EncryptionKeyPair(spk)
	if err != nil {
		t.Fatal(err)
	}

	amount, token, stealth, ts := sampleUtxoFields()
	sealed, err := cryptokeys.EncryptSealedKeys(buildUtxoPlaintext(amount, token, stealth, ts), []*[32]byte{&pk})
	if err != nil {
		t.Fatal(err)
	}
	encrypted := append(balance.SealedKeysPrefix(), sealed...)

	u, err := balance.DecryptUtxo(encrypted, uk)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	assertEqual(t, "amount", u.Amount.String(), amount.String())
	assertEqual(t, "token", u.Erc20TokenAddress, token)
	assertEqual(t, "stealth", u.StealthAddress, stealth)
	assertEqual(t, "timestamp", u.TimeStamp, ts.String())
	assertEqual(t, "isNewStyle", u.IsNewStyle, true)

	if c, err := u.GetCommitment(); err != nil || c == "" {
		t.Fatalf("commitment: %q err=%v", c, err)
	}
	if n, err := u.GetNullifier(); err != nil || n == "" {
		t.Fatalf("nullifier: %q err=%v", n, err)
	}
}

func TestDecryptUtxo_LegacyBoxSealRoundTrip(t *testing.T) {
	uk := cryptokeys.NewUserKeys("0x" + strings.Repeat("cd", 65))
	spk, _ := uk.GetShieldedPrivateKey()
	_, pk, _ := cryptokeys.EncryptionKeyPair(spk)

	amount, token, stealth, ts := sampleUtxoFields()
	sealed, err := box.SealAnonymous(nil, buildUtxoPlaintext(amount, token, stealth, ts), &pk, nil)
	if err != nil {
		t.Fatal(err)
	}

	u, err := balance.DecryptUtxo(sealed, uk)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	assertEqual(t, "amount", u.Amount.String(), amount.String())
	assertEqual(t, "token", u.Erc20TokenAddress, token)
}

func TestDecryptUtxo_WrongKeyFails(t *testing.T) {
	owner := cryptokeys.NewUserKeys("0x" + strings.Repeat("ab", 65))
	other := cryptokeys.NewUserKeys("0x" + strings.Repeat("ef", 65))

	spk, _ := owner.GetShieldedPrivateKey()
	_, pk, _ := cryptokeys.EncryptionKeyPair(spk)
	amount, token, stealth, ts := sampleUtxoFields()
	sealed, _ := cryptokeys.EncryptSealedKeys(buildUtxoPlaintext(amount, token, stealth, ts), []*[32]byte{&pk})
	encrypted := append(balance.SealedKeysPrefix(), sealed...)

	if _, err := balance.DecryptUtxo(encrypted, other); err == nil {
		t.Fatal("expected decryption to fail for a non-recipient key")
	}
}

func TestDecodeEvmUtxo_NegativeOldStyle(t *testing.T) {
	uk := cryptokeys.NewUserKeysFromNullifyingKey("0x" + strings.Repeat("01", 32))
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	amount := big.NewInt(42_000)
	token := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	randomization := big.NewInt(12345)
	h0x, h0y, h1y, stealth := oldStyleFields(t, randomization, spk)
	encoded := buildAbiEncodedEvmUtxo(amount, token, h0x, stealth, h0y, h1y, big.NewInt(1_700_000_123), false)

	u, err := balance.DecodeEvmUtxoHex(encoded, uk)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertEqual(t, "amount", u.Amount.String(), amount.String())
	assertEqual(t, "token", u.Erc20TokenAddress, token.Hex())
	assertEqual(t, "isNewStyle", u.IsNewStyle, false)

	res, err := balance.Compute(
		[]*types.EncryptedOutputWithSign{{Value: encoded, IsPositive: false}},
		map[string]struct{}{},
		uk,
		constants.ChainIDs.EthMainnet,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Balances[token.Hex()]; got == nil || got.Cmp(amount) != 0 {
		t.Fatalf("balance = %v, want %s", got, amount)
	}

	wrong := cryptokeys.NewUserKeysFromNullifyingKey("0x" + strings.Repeat("02", 32))
	if _, err := balance.DecodeEvmUtxoHex(encoded, wrong); err == nil {
		t.Fatal("expected wrong key to fail ownership check")
	}
}

func TestDecodeEvmUtxo_NegativeNewStyle(t *testing.T) {
	uk := cryptokeys.NewUserKeysFromNullifyingKey("0x" + strings.Repeat("03", 32))
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	amount := big.NewInt(77_000)
	token := common.HexToAddress("0x00000000000000000000000000000000000000bb")
	randomization := big.NewInt(98765)
	h0 := babyjub.NewPoint().Mul(randomization, babyjub.B8)
	h1 := h1FromH0(t, h0, spk)
	encoded := buildAbiEncodedEvmUtxo(amount, token, h0.X, h1.Y, h0.Y, h1.Y, big.NewInt(1_700_000_456), true)

	u, err := balance.DecodeEvmUtxoHex(encoded, uk)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertEqual(t, "amount", u.Amount.String(), amount.String())
	assertEqual(t, "token", u.Erc20TokenAddress, token.Hex())
	assertEqual(t, "isNewStyle", u.IsNewStyle, true)
}

func TestDecodeSolanaOnChainUtxo(t *testing.T) {
	uk := cryptokeys.NewUserKeysFromNullifyingKey("0x" + strings.Repeat("04", 32))
	spk, err := uk.GetShieldedPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	amount := big.NewInt(88_000)
	randomization := big.NewInt(45678)
	h0 := babyjub.NewPoint().Mul(randomization, babyjub.B8)
	h1 := h1FromH0(t, h0, spk)
	mintBytes := []byte{
		0, 1, 2, 3, 4, 5, 6, 7,
		8, 9, 10, 11, 12, 13, 14, 15,
		16, 17, 18, 19, 20, 21, 22, 23,
		24, 25, 26, 27, 28, 29, 30, 31,
	}
	mintPart1 := append(make([]byte, 16), mintBytes[:16]...)
	mintPart2 := append(make([]byte, 16), mintBytes[16:]...)
	rawH0x := new(big.Int).Add(h0.X, testHighestBit())
	encoded, err := utils.EncodeSolanaOnChainUtxo([][]byte{
		wordBytes(amount),
		mintPart1,
		mintPart2,
		wordBytes(rawH0x),
		wordBytes(h1.Y),
		wordBytes(h0.Y),
		wordBytes(h1.Y),
		wordBytes(big.NewInt(1_700_000_789)),
	})
	if err != nil {
		t.Fatal(err)
	}

	u, err := balance.DecodeSolanaOnChainUtxo(encoded, uk)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	mint := base58.Encode(mintBytes)
	assertEqual(t, "amount", u.Amount.String(), amount.String())
	assertEqual(t, "mint", u.MintAddress, mint)
	assertEqual(t, "isNewStyle", u.IsNewStyle, true)

	res, err := balance.Compute(
		[]*types.EncryptedOutputWithSign{{Value: encoded, IsPositive: false}},
		map[string]struct{}{},
		uk,
		constants.ChainIDs.SolanaMainnet,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Balances[mint]; got == nil || got.Cmp(amount) != 0 {
		t.Fatalf("balance = %v, want %s", got, amount)
	}
}

func TestEVMDecodeFromChain_Live(t *testing.T) {
	requireLive(t)
	rpcURL, err := constants.FetchRPCURL(constants.ChainIDs.EthMainnet)
	if err != nil {
		t.Skip("ALCHEMY_API_KEY not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	raw, err := api.FetchSnapshots(ctx, constants.ChainIDs.EthMainnet)
	if err != nil {
		t.Fatalf("fetch snapshots: %v", err)
	}
	emitter, err := eventservice.NewEVMEmitter(constants.ChainIDs.EthMainnet, rpcURL, raw.Commitments.HinkalAddress, 0, nil)
	if err != nil {
		t.Fatalf("evm emitter: %v", err)
	}
	to, err := emitter.GetLastBlockNumberForEventRequest(ctx)
	if err != nil {
		t.Fatalf("last block: %v", err)
	}
	from := to - 20000
	events, err := emitter.GetEventsInRange(ctx, from, to)
	if err != nil {
		t.Fatalf("get events: %v", err)
	}

	newCommitments := 0
	for _, ev := range events {
		if ev.EventName != "NewCommitment" {
			continue
		}
		newCommitments++
		enc, err := ev.GetArg("encryptedOutput")
		if err != nil {
			t.Fatalf("getArg encryptedOutput: %v", err)
		}
		if !strings.HasPrefix(enc, "0x") {
			t.Fatalf("encryptedOutput not 0x-hex: %q", enc)
		}
		if _, err := hex.DecodeString(enc[2:]); err != nil {
			t.Fatalf("encryptedOutput not valid hex: %v", err)
		}
		c, err := ev.GetArg("commitment")
		if err != nil {
			t.Fatalf("getArg commitment: %v", err)
		}
		if _, err := utils.ParseBigInt(c); err != nil {
			t.Fatalf("commitment not parseable: %v", err)
		}
	}
	t.Logf("decoded %d NewCommitment events in blocks %d..%d", newCommitments, from, to)
	if newCommitments == 0 {
		t.Skip("no NewCommitment events in scanned range")
	}
}

func buildAbiEncodedEvmUtxo(
	amount *big.Int,
	token common.Address,
	h0x *big.Int,
	stealth *big.Int,
	h0y *big.Int,
	h1y *big.Int,
	ts *big.Int,
	isNewStyle bool,
) string {
	var out []byte
	out = appendWord(out, amount)
	out = appendAddressWord(out, token)
	rawH0x := new(big.Int).Set(h0x)
	if isNewStyle {
		rawH0x.Add(rawH0x, testHighestBit())
	}
	out = appendWord(out, rawH0x)
	out = appendWord(out, stealth)
	out = appendWord(out, h0y)
	out = appendWord(out, h1y)
	out = appendWord(out, ts)
	out = appendWord(out, big.NewInt(0))
	return "0x" + hex.EncodeToString(out)
}

func oldStyleFields(t *testing.T, randomization *big.Int, privateKey string) (*big.Int, *big.Int, *big.Int, *big.Int) {
	t.Helper()
	privateKeyAdjusted := adjustedPrivateKey(t, privateKey)
	multiplier := new(big.Int).Mul(randomization, privateKeyAdjusted)
	multiplier.Mod(multiplier, crypto.FieldP)
	h0 := babyjub.NewPoint().Mul(randomization, babyjub.B8)
	h1 := babyjub.NewPoint().Mul(multiplier, babyjub.B8)
	signs := big.NewInt(int64(2*testCircomSign(h0.X) + testCircomSign(h1.X)))
	stealth, err := crypto.PoseidonBig(signs, h0.Y, h1.Y)
	if err != nil {
		t.Fatal(err)
	}
	return randomization, testCompressedPointBigInt(h0), testCompressedPointBigInt(h1), stealth
}

func h1FromH0(t *testing.T, h0 *babyjub.Point, privateKey string) *babyjub.Point {
	t.Helper()
	return babyjub.NewPoint().Mul(adjustedPrivateKey(t, privateKey), h0)
}

func adjustedPrivateKey(t *testing.T, privateKey string) *big.Int {
	t.Helper()
	n, err := utils.ParseBigInt(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return new(big.Int).Mod(n, crypto.FieldP)
}

func testCompressedPointBigInt(p *babyjub.Point) *big.Int {
	out := new(big.Int).Set(p.Y)
	if testCircomSign(p.X) == 1 {
		out.Add(out, testHighestBit())
	}
	return out
}

func testCircomSign(n *big.Int) int {
	half := new(big.Int).Rsh(new(big.Int).Set(crypto.FieldP), 1)
	if n.Cmp(half) == 1 {
		return 1
	}
	return 0
}

func testHighestBit() *big.Int {
	return new(big.Int).Lsh(big.NewInt(1), 255)
}

func appendWord(out []byte, n *big.Int) []byte {
	return append(out, wordBytes(n)...)
}

func appendAddressWord(out []byte, addr common.Address) []byte {
	word := make([]byte, 32)
	copy(word[12:], addr.Bytes())
	return append(out, word...)
}

func wordBytes(n *big.Int) []byte {
	word := make([]byte, 32)
	n.FillBytes(word)
	return word
}

// HINKAL_LIVE=1 HINKAL_SIGNATURE=0x... [HINKAL_CHAIN_ID=1] go test ./tests/... -run TestBalance_Live -v
func TestBalance_Live(t *testing.T) {
	requireLive(t)
	sig := liveSignature(t)
	chainID := liveChainID(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	encOutputs, nullifierSet, snapshotBlock := loadChainState(t, ctx, chainID)

	uk := cryptokeys.NewUserKeys(sig)
	res, err := balance.Compute(encOutputs, nullifierSet, uk, chainID)
	if err != nil {
		t.Fatalf("compute balance: %v", err)
	}

	t.Logf("chain=%d snapshotBlock=%d outputs=%d nullifiers=%d ownedUtxos=%d",
		chainID, snapshotBlock, len(encOutputs), len(nullifierSet), len(res.Utxos))
	if len(res.Balances) == 0 {
		t.Logf("no balance for this signature on chain %d", chainID)
	}
	for token, amt := range res.Balances {
		t.Logf("balance %s = %s", token, amt.String())
	}
}
