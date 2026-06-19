package merkletree

import (
	"math/big"

	"github.com/gioeba/go_sdk_test/types"
)

type HashFunc func(a, b *big.Int) *big.Int

type MerkleTree interface {
	Insert(value, index *big.Int)
	Remove(index *big.Int)
	GetValue(index *big.Int) (*big.Int, bool)
	GetStartIndex() *big.Int
	LastLeaves(limit int) []*big.Int
	GetRootHash() (*big.Int, error)
	GetSiblingHashesForVerification(item *big.Int) ([]*big.Int, error)
	GetSiblingSides(item *big.Int) ([]*big.Int, error)
	ToJSON() types.MerkleTreeJSON
	Clone() MerkleTree
}
