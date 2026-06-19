package types

type TokenWithID struct {
	Erc20TokenAddress string `json:"erc20TokenAddress"`
	TokenID           int    `json:"tokenId"`
}
