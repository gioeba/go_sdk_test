package balance

import (
	"context"
	"runtime"
	"sync"

	"github.com/gioeba/go_sdk_test/internal/cache"
	"github.com/gioeba/go_sdk_test/cryptokeys"
	"github.com/gioeba/go_sdk_test/data-structures/hinkal/ihinkal"
	"github.com/gioeba/go_sdk_test/internal/functions/enclave"
	"github.com/gioeba/go_sdk_test/types"
	"github.com/gioeba/go_sdk_test/internal/utxo"
)

type InputUtxoParams struct {
	Hinkal                  ihinkal.HinkalInternal
	ChainID                 int
	PassedShieldedPublicKey string
	EthAddress              string
	ResetCacheBefore        bool
	AllowRemoteDecryption   bool
	UseBlockedUtxos         bool
}

var localFetchingLock sync.Mutex

func getInputUtxosLocally(
	hinkal ihinkal.HinkalInternal,
	userKeys *cryptokeys.UserKeys,
	currentChainID int,
	shieldedPublicKey string,
	hinkalOutputs []*types.EncryptedOutputWithSign,
	cachedEncryptedOutputs []*types.EncryptedOutputWithSign,
	lastOutput string,
	lastOutputIndex int,
) ([]*utxo.Utxo, error) {
	localFetchingLock.Lock()
	defer localFetchingLock.Unlock()

	lastOutputIndex++
	var outputsSlice []*types.EncryptedOutputWithSign
	if lastOutputIndex < len(hinkalOutputs) {
		outputsSlice = hinkalOutputs[lastOutputIndex:]
	}

	additionalEncryptedOutputs, err := filterOwnedOutputs(outputsSlice, userKeys, currentChainID)
	if err != nil {
		return nil, err
	}

	lastOutputNew := lastOutput
	if len(hinkalOutputs) > 0 {
		lastOutputNew = hinkalOutputs[len(hinkalOutputs)-1].Value
	}

	newEncryptedOutputs := make([]*types.EncryptedOutputWithSign, 0, len(cachedEncryptedOutputs)+len(additionalEncryptedOutputs))
	newEncryptedOutputs = append(newEncryptedOutputs, cachedEncryptedOutputs...)
	newEncryptedOutputs = append(newEncryptedOutputs, additionalEncryptedOutputs...)

	if err := cache.SetHinkalCache(
		cache.HinkalCacheInterface{LastOutput: lastOutputNew, EncryptedOutputs: newEncryptedOutputs},
		hinkal,
		currentChainID,
		shieldedPublicKey,
	); err != nil {
		return nil, err
	}

	return DecodeOutputs(newEncryptedOutputs, userKeys, currentChainID)
}

func getInputUtxosRemotely(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	userKeys *cryptokeys.UserKeys,
	currentChainID int,
	shieldedPublicKey string,
) ([]*utxo.Utxo, error) {
	resolvedUtxos, newEncryptedOutputs, lastOutput, err := enclave.GetInputUtxosEnclave(ctx, currentChainID, userKeys)
	if err != nil {
		return nil, err
	}

	if err := cache.SetHinkalCache(
		cache.HinkalCacheInterface{EncryptedOutputs: newEncryptedOutputs, LastOutput: lastOutput},
		hinkal,
		currentChainID,
		shieldedPublicKey,
	); err != nil {
		return nil, err
	}

	return resolvedUtxos, nil
}

func attemptGetInputUtxosRemotely(
	ctx context.Context,
	hinkal ihinkal.HinkalInternal,
	userKeys *cryptokeys.UserKeys,
	currentChainID int,
	shieldedPublicKey string,
	hinkalOutputs []*types.EncryptedOutputWithSign,
	cachedEncryptedOutputs []*types.EncryptedOutputWithSign,
	lastOutput string,
	lastOutputIndex int,
) ([]*utxo.Utxo, error) {
	resolvedUtxos, err := getInputUtxosRemotely(ctx, hinkal, userKeys, currentChainID, shieldedPublicKey)
	if err != nil {
		return getInputUtxosLocally(hinkal, userKeys, currentChainID, shieldedPublicKey, hinkalOutputs, cachedEncryptedOutputs, lastOutput, lastOutputIndex)
	}
	return resolvedUtxos, nil
}

func GetInputUtxoAndBalance(ctx context.Context, p InputUtxoParams) ([]*utxo.Utxo, error) {
	hinkal := p.Hinkal
	chainID := p.ChainID
	userKeys := hinkal.GetUserKeys()

	shieldedPublicKey := p.PassedShieldedPublicKey
	if shieldedPublicKey == "" {
		spk, err := userKeys.GetShieldedPublicKey()
		if err != nil {
			return nil, err
		}
		shieldedPublicKey = spk
	}

	encryptedOutputs := hinkal.EncryptedOutputs(chainID)
	nullifiers := hinkal.Nullifiers(chainID)
	hinkalOutputs := encryptedOutputs

	if p.ResetCacheBefore {
		if err := cache.ResetCache(hinkal, chainID, shieldedPublicKey); err != nil {
			return nil, err
		}
	}

	cached, err := cache.GetHinkalCache(hinkal, chainID, shieldedPublicKey)
	if err != nil {
		return nil, err
	}
	cachedEncryptedOutputs := cached.EncryptedOutputs
	lastOutput := cached.LastOutput

	lastOutputExist := lastOutput != ""
	lastOutputIndex := indexOfOutput(hinkalOutputs, lastOutput)

	case1 := lastOutputIndex > -1
	case2 := !lastOutputExist && lastOutputIndex == -1

	forceReadFromCache := hinkal.AreMerkleTreeUpdatesDisabled()

	decryptLocally := !forceReadFromCache && (case1 || (!p.AllowRemoteDecryption && case2))
	decryptRemotely := !forceReadFromCache && p.AllowRemoteDecryption && case2
	useCache := !decryptLocally && !decryptRemotely

	var allUtxos []*utxo.Utxo

	switch {
	case decryptLocally:
		allUtxos, err = getInputUtxosLocally(hinkal, userKeys, chainID, shieldedPublicKey, hinkalOutputs, cachedEncryptedOutputs, lastOutput, lastOutputIndex)
	case decryptRemotely:
		allUtxos, err = attemptGetInputUtxosRemotely(ctx, hinkal, userKeys, chainID, shieldedPublicKey, hinkalOutputs, cachedEncryptedOutputs, lastOutput, lastOutputIndex)
	case useCache:
		allUtxos, err = DecodeOutputs(cachedEncryptedOutputs, userKeys, chainID)
	}
	if err != nil {
		return nil, err
	}

	inputUtxos := filterSpentUtxos(allUtxos, nullifiers)

	filteredInputUtxos := make([]*utxo.Utxo, 0, len(inputUtxos))
	for _, u := range inputUtxos {
		if u.IsBlocked == p.UseBlockedUtxos {
			filteredInputUtxos = append(filteredInputUtxos, u)
		}
	}
	return filteredInputUtxos, nil
}

func DecodeOutputs(encryptedOutputs []*types.EncryptedOutputWithSign, uk *cryptokeys.UserKeys, chainID int) ([]*utxo.Utxo, error) {
	utxos := make([]*utxo.Utxo, 0, len(encryptedOutputs))
	for _, out := range encryptedOutputs {
		u, err := decodeOutput(out, uk, chainID)
		if err != nil {
			continue
		}
		u.IsBlocked = out.IsBlocked
		utxos = append(utxos, u)
	}
	return utxos, nil
}

const parallelDecryptThreshold = 256

func filterOwnedOutputs(outputs []*types.EncryptedOutputWithSign, uk *cryptokeys.UserKeys, chainID int) ([]*types.EncryptedOutputWithSign, error) {
	if _, err := uk.GetShieldedPrivateKey(); err != nil {
		return nil, err
	}

	ownedFlags := make([]bool, len(outputs))

	if len(outputs) < parallelDecryptThreshold {
		for i, out := range outputs {
			if _, err := decodeOutput(out, uk, chainID); err == nil {
				ownedFlags[i] = true
			}
		}
	} else {
		workers := runtime.NumCPU()
		if workers > len(outputs) {
			workers = len(outputs)
		}
		chunk := (len(outputs) + workers - 1) / workers

		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			start := w * chunk
			if start >= len(outputs) {
				break
			}
			end := start + chunk
			if end > len(outputs) {
				end = len(outputs)
			}
			wg.Add(1)
			go func(start, end int) {
				defer wg.Done()
				for i := start; i < end; i++ {
					if _, err := decodeOutput(outputs[i], uk, chainID); err == nil {
						ownedFlags[i] = true
					}
				}
			}(start, end)
		}
		wg.Wait()
	}

	owned := make([]*types.EncryptedOutputWithSign, 0, len(outputs))
	for i, isOwned := range ownedFlags {
		if isOwned {
			owned = append(owned, outputs[i])
		}
	}
	return owned, nil
}

func filterSpentUtxos(allUtxos []*utxo.Utxo, nullifiers map[string]struct{}) []*utxo.Utxo {
	out := make([]*utxo.Utxo, 0, len(allUtxos))
	for _, u := range allUtxos {
		nul, err := u.GetNullifier()
		if err != nil {
			continue
		}
		if _, spent := nullifiers[nul]; spent {
			continue
		}
		out = append(out, u)
	}
	return out
}

func indexOfOutput(outputs []*types.EncryptedOutputWithSign, value string) int {
	for i, out := range outputs {
		if out.Value == value {
			return i
		}
	}
	return -1
}
