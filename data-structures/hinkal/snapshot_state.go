package hinkal

import (
	"github.com/gioeba/go_sdk_test/data-structures/merkletree"
	"github.com/gioeba/go_sdk_test/types"
)

type commitmentsSnapshot interface {
	EncryptedOutputs() []*types.EncryptedOutputWithSign
	MerkleTree() merkletree.MerkleTree
}

type nullifierSnapshot interface {
	Nullifiers() map[string]struct{}
}
