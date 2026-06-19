package types

import "math/big"

type CommitmentEvent struct {
	Commitment      *big.Int
	Index           *big.Int
	EncryptedOutput string
}

type NullifierEvent struct {
	Nullifier string
}

type EncryptedOutputWithSign struct {
	Value      string
	IsPositive bool
	IsBlocked  bool
}

type EncryptedOutputWithCommitment struct {
	Commitment      string
	EncryptedOutput string
}

type EventCategory string

const (
	EventCategoryMain        EventCategory = "MainContractEvents"
	EventCategoryAccessToken EventCategory = "AccessTokenContractEvents"
)
