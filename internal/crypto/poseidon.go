package crypto

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/iden3/go-iden3-crypto/poseidon"

	"github.com/gioeba/go_sdk_test/data-structures/merkletree"
)

func PoseidonHash(a, b *big.Int) *big.Int {
	result, err := poseidon.Hash([]*big.Int{a, b})
	if err != nil {
		panic(fmt.Sprintf("poseidon hash failed: %v", err))
	}
	return result
}

var FieldP = fr.Modulus()

func PoseidonBig(inputs ...*big.Int) (*big.Int, error) {
	reduced := make([]*big.Int, len(inputs))
	for i, in := range inputs {
		reduced[i] = new(big.Int).Mod(in, FieldP)
	}
	return poseidon.Hash(reduced)
}

var PoseidonHashFunc merkletree.HashFunc = PoseidonHash
