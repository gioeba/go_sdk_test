package snarkjs

import (
	"context"
	"errors"
	"math/big"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/data-structures/merkletree"
	"github.com/gioeba/go_sdk_test/internal/functions/utils"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

type MerkleDataFromWorkers struct {
	InCommitmentSiblings     [][][]string
	InCommitmentSiblingSides [][][]string
	RootHashHinkal           *big.Int
	InNullifiers             [][]string
}

type merkleTreeSiblingsAndRootHashes struct {
	inCommitmentSiblings     [][][]string
	inCommitmentSiblingSides [][][]string
	rootHashHinkal           *big.Int
}

// zero-amount UTXOs nullify to "0" (worker handleBuildInNullifiers semantics, which zeroes by
// amount, not by onChainCreation).
func buildInNullifiersByAmount(inputUtxos [][]*utxo.Utxo) ([][]string, error) {
	out := make([][]string, len(inputUtxos))
	for i, token := range inputUtxos {
		out[i] = make([]string, len(token))
		for j, u := range token {
			if u.Amount.Sign() == 0 {
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

func handleLocalMerkleTrees(merkleTree merkletree.MerkleTree, inputUtxos [][]*utxo.Utxo) (merkleTreeSiblingsAndRootHashes, error) {
	if merkleTree == nil {
		return merkleTreeSiblingsAndRootHashes{}, errors.New("root hash not available from hinkal merkle tree")
	}
	rootHash, err := merkleTree.GetRootHash()
	if err != nil {
		return merkleTreeSiblingsAndRootHashes{}, err
	}
	siblings, sides, err := CalcCommitmentsSiblingAndSides(inputUtxos, merkleTree)
	if err != nil {
		return merkleTreeSiblingsAndRootHashes{}, err
	}
	return merkleTreeSiblingsAndRootHashes{
		inCommitmentSiblings:     siblings,
		inCommitmentSiblingSides: sides,
		rootHashHinkal:           rootHash,
	}, nil
}

func handleRemoteMerkleTrees(ctx context.Context, chainID int, inputUtxos [][]*utxo.Utxo) (merkleTreeSiblingsAndRootHashes, error) {
	resp, err := FetchMerkleTreeSiblings(ctx, chainID, inputUtxos)
	if err != nil {
		return merkleTreeSiblingsAndRootHashes{}, err
	}
	rootHash, err := utils.ParseBigInt(resp.RootHashHinkal)
	if err != nil {
		return merkleTreeSiblingsAndRootHashes{}, err
	}
	return merkleTreeSiblingsAndRootHashes{
		inCommitmentSiblings:     resp.InCommitmentSiblings,
		inCommitmentSiblingSides: resp.InCommitmentSiblingSides,
		rootHashHinkal:           rootHash,
	}, nil
}

func areLocalTreesUpToDate(ctx context.Context, chainID int, merkleTree merkletree.MerkleTree) bool {
	if merkleTree == nil {
		return false
	}
	localRoot, err := merkleTree.GetRootHash()
	if err != nil {
		return false
	}
	onChainRoot, err := FetchOnChainRootHashes(ctx, chainID)
	if err != nil {
		return false
	}
	return localRoot.Cmp(onChainRoot) == 0
}

func GetMerkleTreeSiblingsAndRootHashes(ctx context.Context, chainID int, merkleTree merkletree.MerkleTree, inputUtxos [][]*utxo.Utxo) (merkleTreeSiblingsAndRootHashes, error) {
	if areLocalTreesUpToDate(ctx, chainID, merkleTree) || constants.IsLocalNetwork(chainID) {
		return handleLocalMerkleTrees(merkleTree, inputUtxos)
	}
	return handleRemoteMerkleTrees(ctx, chainID, inputUtxos)
}

func GetDataFromWorkers(ctx context.Context, chainID int, merkleTree merkletree.MerkleTree, inputUtxos [][]*utxo.Utxo) (MerkleDataFromWorkers, error) {
	siblings, err := GetMerkleTreeSiblingsAndRootHashes(ctx, chainID, merkleTree, inputUtxos)
	if err != nil {
		return MerkleDataFromWorkers{}, err
	}

	nullifiers, err := buildInNullifiersByAmount(inputUtxos)
	if err != nil {
		return MerkleDataFromWorkers{}, err
	}

	return MerkleDataFromWorkers{
		InCommitmentSiblings:     siblings.inCommitmentSiblings,
		InCommitmentSiblingSides: siblings.inCommitmentSiblingSides,
		RootHashHinkal:           siblings.rootHashHinkal,
		InNullifiers:             nullifiers,
	}, nil
}
