package constants

import "fmt"

// FetchRPCURL returns a chain's fetch RPC URL from EthereumNetworkRegistry — the single source of
// truth for RPC endpoints.
func FetchRPCURL(chainID int) (string, error) {
	net, ok := EthereumNetworkRegistry[chainID]
	if !ok || net.FetchRPCURL == "" {
		return "", fmt.Errorf("no fetch rpc url configured for chain %d", chainID)
	}
	return net.FetchRPCURL, nil
}
