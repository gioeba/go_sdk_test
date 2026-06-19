package transactions

import (
	"math/big"
	"testing"

	"github.com/gioeba/go_sdk_test/constants"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

func TestTopPositiveUtxosForTokenSortsLargestSix(t *testing.T) {
	tokenAddress := "0x3600000000000000000000000000000000000000"
	otherTokenAddress := "0x1111111111111111111111111111111111111111"
	input := []*utxo.Utxo{
		{Amount: big.NewInt(1), Erc20TokenAddress: tokenAddress},
		{Amount: big.NewInt(8), Erc20TokenAddress: tokenAddress},
		{Amount: big.NewInt(3), Erc20TokenAddress: tokenAddress},
		{Amount: big.NewInt(0), Erc20TokenAddress: tokenAddress},
		{Amount: big.NewInt(5), Erc20TokenAddress: tokenAddress},
		{Amount: big.NewInt(2), Erc20TokenAddress: tokenAddress},
		{Amount: big.NewInt(7), Erc20TokenAddress: tokenAddress},
		{Amount: big.NewInt(6), Erc20TokenAddress: tokenAddress},
		{Amount: big.NewInt(100), Erc20TokenAddress: otherTokenAddress},
		{Amount: big.NewInt(-10), Erc20TokenAddress: tokenAddress},
	}

	selected, err := topPositiveUtxosForToken(input, constants.ChainIDs.ArcTestnet, tokenAddress)
	if err != nil {
		t.Fatalf("top positive utxos: %v", err)
	}

	want := []int64{8, 7, 6, 5, 3, 2}
	if len(selected) != len(want) {
		t.Fatalf("selected length = %d, want %d", len(selected), len(want))
	}
	for i, amount := range want {
		if selected[i].Amount.Cmp(big.NewInt(amount)) != 0 {
			t.Fatalf("selected[%d] = %s, want %d", i, selected[i].Amount, amount)
		}
	}
}
