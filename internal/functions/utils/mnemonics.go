package utils

import (
	"encoding/hex"
	"strings"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// GenerateHashFromSeedPhrases mirrors @hinkal/common's generateHashFromSeedPhrases:
// keccak256 over the UTF-8 bytes of the space-joined seed phrases, 0x-prefixed.
func GenerateHashFromSeedPhrases(seedPhrases []string) string {
	seedPhrasesString := strings.Join(seedPhrases, " ")
	hash := gethcrypto.Keccak256([]byte(seedPhrasesString))
	return "0x" + hex.EncodeToString(hash)
}
