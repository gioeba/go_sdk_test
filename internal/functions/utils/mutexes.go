package utils

import "sync"

var chainMutexes sync.Map

func GetChainBalanceFetchingMutex(chainID int) *sync.Mutex {
	mutex, _ := chainMutexes.LoadOrStore(chainID, &sync.Mutex{})
	m, ok := mutex.(*sync.Mutex)
	if !ok {
		panic("chain mutex registry holds a non-*sync.Mutex value")
	}
	return m
}
