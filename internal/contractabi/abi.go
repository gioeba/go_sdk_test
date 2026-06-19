package contractabi

import (
	"bytes"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/gioeba/go_sdk_test/constants"
)

var (
	hinkalABIMu    sync.Mutex
	hinkalABICache = map[int]abi.ABI{}
)

func Hinkal(chainID int) (abi.ABI, error) {
	hinkalABIMu.Lock()
	defer hinkalABIMu.Unlock()

	if cached, ok := hinkalABICache[chainID]; ok {
		return cached, nil
	}
	raw, err := constants.HinkalABIJSON(chainID)
	if err != nil {
		return abi.ABI{}, err
	}
	parsed, err := abi.JSON(bytes.NewReader(raw))
	if err != nil {
		return abi.ABI{}, err
	}
	hinkalABICache[chainID] = parsed
	return parsed, nil
}
