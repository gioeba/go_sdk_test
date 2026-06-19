package snarkjs

import (
	"context"

	"github.com/gioeba/go_sdk_test/internal/api"
	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

type MerkleTreeSiblingsResponse struct {
	InCommitmentSiblings     [][][]string `json:"inCommitmentSiblings"`
	InCommitmentSiblingSides [][][]string `json:"inCommitmentSiblingSides"`
	AccessTokenSiblings      []string     `json:"accessTokenSiblings"`
	AccessTokenSiblingSides  []string     `json:"accessTokenSiblingSides"`
	RootHashHinkal           string       `json:"rootHashHinkal"`
	RootHashAccessToken      string       `json:"rootHashAccessToken"`
}

func FetchMerkleTreeSiblings(ctx context.Context, chainID int, inputUtxos [][]*utxo.Utxo) (*MerkleTreeSiblingsResponse, error) {
	serialized := make([][]types.UtxoParams, len(inputUtxos))
	for i, token := range inputUtxos {
		serialized[i] = make([]types.UtxoParams, len(token))
		for j, u := range token {
			serialized[i][j] = u.GetConstructableParams()
		}
	}

	body := map[string]any{
		"inputUtxosSerialized": serialized,
		"chainId":              chainID,
	}

	url := constants.GetSnapshotServerURL() + constants.SnapshotServerConfig.MerkleTreeSiblings
	var resp MerkleTreeSiblingsResponse
	if err := api.Post(ctx, url, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
