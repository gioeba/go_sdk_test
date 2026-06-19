package snarkjs

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/crypto"
	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/data-structures/merkletree"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

var circomP = crypto.FieldP

func GetExternalActionIDHash(externalActionID types.ExternalActionID) *big.Int {
	if externalActionID == types.ExternalActionZero {
		return big.NewInt(0)
	}
	hash := gethcrypto.Keccak256([]byte(string(externalActionID)))
	n := new(big.Int).SetBytes(hash)
	return n.Mod(n, circomP)
}

func GetOriginalSender(externalAddress, relay string) string {
	if relay == constants.ZeroAddress {
		if externalAddress == "" {
			return constants.ZeroAddress
		}
		return externalAddress
	}
	return constants.ZeroAddress
}

func GetUtxoCircuitH0Coords(u *utxo.Utxo) (types.JubPoint, error) {
	if u.H0 != nil && (*u.H0)[0] != nil && (*u.H0)[1] != nil {
		return *u.H0, nil
	}
	if u.Randomization != nil {
		return cryptokeys.GetH0FromRandomization(u.Randomization), nil
	}
	return types.JubPoint{}, errors.New("snarkjs: UTXO missing H0 coordinates for proof input")
}

func GetUtxoCircuitInRandomization(u *utxo.Utxo) (string, error) {
	// New-style UTXOs don't use randomization (they use H0 instead). The circuit still runs an old
	// size-check on it, and a stored randomization can be too big and fail that check. Since it's
	// unused here, send 0 — always valid.
	if u.IsNewStyle {
		return "0", nil
	}
	if u.Randomization != nil {
		return u.Randomization.String(), nil
	}
	return "", errors.New("snarkjs: UTXO missing randomization for old-style proof input")
}

func BuildInNullifiers(inputUtxos [][]*utxo.Utxo, onChainCreation []bool) ([][]string, error) {
	out := make([][]string, len(inputUtxos))
	for i, token := range inputUtxos {
		out[i] = make([]string, len(token))
		for j, u := range token {
			if onChainCreation[i] {
				out[i][j] = "0"
				continue
			}
			n, err := u.GetNullifier()
			if err != nil {
				return nil, err
			}
			out[i][j] = n
		}
	}
	return out, nil
}

func hasOnlyZeroAmounts(inputUtxos [][]*utxo.Utxo) bool {
	if len(inputUtxos) == 0 {
		return false
	}
	for _, tokenUtxos := range inputUtxos {
		for _, u := range tokenUtxos {
			if u.Amount.Sign() != 0 {
				return false
			}
		}
	}
	return true
}

func BuildOutCommitments(outputUtxos [][]*utxo.Utxo) ([][]string, error) {
	out := make([][]string, len(outputUtxos))
	for i, token := range outputUtxos {
		out[i] = make([]string, len(token))
		for j, u := range token {
			if u.Amount.Sign() == 0 {
				out[i][j] = "0"
				continue
			}
			c, err := u.GetCommitment()
			if err != nil {
				return nil, err
			}
			out[i][j] = c
		}
	}
	return out, nil
}

func CalcAmountChanges(inputUtxos, outputUtxos [][]*utxo.Utxo, forCircomData bool) []*big.Int {
	amountChanges := make([]*big.Int, 0, len(inputUtxos))
	for i := range inputUtxos {
		inTotal := big.NewInt(0)
		outTotal := big.NewInt(0)
		for _, u := range inputUtxos[i] {
			inTotal.Add(inTotal, u.Amount)
		}
		if i < len(outputUtxos) {
			for _, u := range outputUtxos[i] {
				outTotal.Add(outTotal, u.Amount)
			}
		}
		diff := new(big.Int).Sub(outTotal, inTotal)
		if diff.Sign() < 0 && !forCircomData {
			diff.Add(circomP, diff)
		}
		amountChanges = append(amountChanges, diff)
	}
	return amountChanges
}

func GetSlippageValues(amountChanges []*big.Int) []*big.Int {
	out := make([]*big.Int, len(amountChanges))
	for i, am := range amountChanges {
		if am.Sign() >= 0 {
			out[i] = big.NewInt(0)
		} else {
			out[i] = new(big.Int).Set(am)
		}
	}
	return out
}

func CalcEncryptedOutputs(outputUtxos [][]*utxo.Utxo) ([][]string, error) {
	if len(outputUtxos) == 0 {
		return [][]string{}, nil
	}
	out := make([][]string, len(outputUtxos))
	for i, token := range outputUtxos {
		out[i] = make([]string, len(token))
		for j, u := range token {
			enc, err := utxo.EncryptUtxo(u)
			if err != nil {
				return nil, err
			}
			out[i][j] = "0x" + hex.EncodeToString(enc)
		}
	}
	return out, nil
}

func CalcCommitmentsSiblingAndSides(
	inputUtxos [][]*utxo.Utxo,
	merkleTree merkletree.MerkleTree,
) (inCommitmentSiblings, inCommitmentSiblingSides [][][]string, err error) {
	inCommitmentSiblings = make([][][]string, len(inputUtxos))
	inCommitmentSiblingSides = make([][][]string, len(inputUtxos))
	for i, token := range inputUtxos {
		inCommitmentSiblings[i] = make([][]string, len(token))
		inCommitmentSiblingSides[i] = make([][]string, len(token))
		for j, u := range token {
			commitment, err := u.GetCommitment()
			if err != nil {
				return nil, nil, err
			}
			commitmentBig, err := utils.ParseBigInt(commitment)
			if err != nil {
				return nil, nil, err
			}
			siblings, err := merkleTree.GetSiblingHashesForVerification(commitmentBig)
			if err != nil {
				return nil, nil, err
			}
			sides, err := merkleTree.GetSiblingSides(commitmentBig)
			if err != nil {
				return nil, nil, err
			}
			inCommitmentSiblings[i][j] = bigIntsToStrings(siblings)
			inCommitmentSiblingSides[i][j] = bigIntsToStrings(sides)
		}
	}
	return inCommitmentSiblings, inCommitmentSiblingSides, nil
}

func CalcStealthAddressStructure(
	extraRandomization *big.Int,
	nullifyingPrivateKey string,
	spendingPublicKey []*big.Int,
) (types.StealthAddressStructure, error) {
	h0, _, err := cryptokeys.GetRandomizedStealthPair(extraRandomization, nullifyingPrivateKey)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	h1, err := cryptokeys.GetH1FromH0(h0, nullifyingPrivateKey)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	stealthStr, err := cryptokeys.GetStealthAddressNewStyle(h0, nullifyingPrivateKey, spendingPublicKey)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	stealthAddress, err := utils.ParseBigInt(stealthStr)
	if err != nil {
		return types.StealthAddressStructure{}, err
	}
	return types.StealthAddressStructure{
		ExtraRandomization: utils.AddHighestBitToNumber(h0[0]),
		H0:                 h0[1],
		H1:                 h1[1],
		StealthAddress:     stealthAddress,
	}, nil
}

func CalcPublicSignalCount(
	verifierName string,
	erc20TokenAddresses []string,
	amountChanges []*big.Int,
	inNullifiers [][]string,
	outCommitments [][]string,
) int {
	if hasPrefix(verifierName, "mainEVMCircuitMin0") {
		return 3
	}
	return 1 + // rootHashHinkal
		1 + // signedMessageHash
		len(erc20TokenAddresses) +
		len(amountChanges) +
		1 + // outTimeStamp
		flatLen(inNullifiers) +
		flatLen(outCommitments) +
		1 + // calldataHash
		1 + // message
		1 + // outH0Ay
		1 + // outH1Ay
		1 + // signs
		1 // outStealthAddress
}

func GetZkProofVerifierName(inputUtxos, outputUtxos [][]*utxo.Utxo) string {
	if len(outputUtxos) == 0 {
		return "mainEVMCircuitMin0"
	}
	return fmt.Sprintf("mainEVMCircuit%dx%dx%d", len(inputUtxos), len(inputUtxos[0]), len(outputUtxos[0]))
}

func bigIntsToStrings(values []*big.Int) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v.String()
	}
	return out
}

func flatLen(values [][]string) int {
	total := 0
	for _, inner := range values {
		total += len(inner)
	}
	return total
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
