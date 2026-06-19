package hinkal

import (
	"context"

	"github.com/gioeba/go_sdk_test/data-structures/merkletree"
	"github.com/gioeba/go_sdk_test/types"
)

type commitmentsSnapshot interface {
	EncryptedOutputs() []*types.EncryptedOutputWithSign
	MerkleTree() merkletree.MerkleTree
	RetrieveEventsFromLatestBlock(ctx context.Context) error
}

type nullifierSnapshot interface {
	Nullifiers() map[string]struct{}
}
