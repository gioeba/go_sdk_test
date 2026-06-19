package snarkjs

import (
	"sync"

	"github.com/gioeba/go_sdk_test/constants"
)

const ProverVersion = "v1x1"

var localVerifiers = map[string]string{
	"mainEVMCircuit1x2x1v1x1Wasm": "mainEVMCircuit1x2x1-1.1.wasm",
	"mainEVMCircuit1x2x1v1x1Zkey": "mainEVMCircuit1x2x1_final-1.1.zkey",
	"mainEVMCircuit1x6x1v1x1Wasm": "mainEVMCircuit1x6x1-1.1.wasm",
	"mainEVMCircuit1x6x1v1x1Zkey": "mainEVMCircuit1x6x1_final-1.1.zkey",
	"mainEVMCircuit2x2x1v1x1Wasm": "mainEVMCircuit2x2x1-1.1.wasm",
	"mainEVMCircuit2x2x1v1x1Zkey": "mainEVMCircuit2x2x1_final-1.1.zkey",
	"mainEVMCircuit2x6x1v1x1Wasm": "mainEVMCircuit2x6x1-1.1.wasm",
	"mainEVMCircuit2x6x1v1x1Zkey": "mainEVMCircuit2x6x1_final-1.1.zkey",
	"mainEVMCircuit3x2x1v1x1Wasm": "mainEVMCircuit3x2x1-1.1.wasm",
	"mainEVMCircuit3x2x1v1x1Zkey": "mainEVMCircuit3x2x1_final-1.1.zkey",
	"mainEVMCircuit3x6x1v1x1Wasm": "mainEVMCircuit3x6x1-1.1.wasm",
	"mainEVMCircuit3x6x1v1x1Zkey": "mainEVMCircuit3x6x1_final-1.1.zkey",
	"mainEVMCircuit4x2x1v1x1Wasm": "mainEVMCircuit4x2x1-1.1.wasm",
	"mainEVMCircuit4x2x1v1x1Zkey": "mainEVMCircuit4x2x1_final-1.1.zkey",
	"mainEVMCircuit4x6x1v1x1Wasm": "mainEVMCircuit4x6x1-1.1.wasm",
	"mainEVMCircuit4x6x1v1x1Zkey": "mainEVMCircuit4x6x1_final-1.1.zkey",
	"mainEVMCircuit5x2x1v1x1Wasm": "mainEVMCircuit5x2x1-1.1.wasm",
	"mainEVMCircuit5x2x1v1x1Zkey": "mainEVMCircuit5x2x1_final-1.1.zkey",
	"mainEVMCircuit5x6x1v1x1Wasm": "mainEVMCircuit5x6x1-1.1.wasm",
	"mainEVMCircuit5x6x1v1x1Zkey": "mainEVMCircuit5x6x1_final-1.1.zkey",
	"mainEVMCircuit1x2x2v1x1Wasm": "mainEVMCircuit1x2x2-1.1.wasm",
	"mainEVMCircuit1x2x2v1x1Zkey": "mainEVMCircuit1x2x2_final-1.1.zkey",
	"mainEVMCircuit1x6x2v1x1Wasm": "mainEVMCircuit1x6x2-1.1.wasm",
	"mainEVMCircuit1x6x2v1x1Zkey": "mainEVMCircuit1x6x2_final-1.1.zkey",
	"mainEVMCircuit2x2x2v1x1Wasm": "mainEVMCircuit2x2x2-1.1.wasm",
	"mainEVMCircuit2x2x2v1x1Zkey": "mainEVMCircuit2x2x2_final-1.1.zkey",
	"mainEVMCircuit2x6x2v1x1Wasm": "mainEVMCircuit2x6x2-1.1.wasm",
	"mainEVMCircuit2x6x2v1x1Zkey": "mainEVMCircuit2x6x2_final-1.1.zkey",
	"mainEVMCircuitMin0v1x1Wasm":  "mainEVMCircuitMin0-1.1.wasm",
	"mainEVMCircuitMin0v1x1Zkey":  "mainEVMCircuitMin0_final-1.1.zkey",

	"mainSolanaCircuit1x2x1v1x1Zkey": "mainSolanaCircuit1x2x1_final-1.1.zkey",
	"mainSolanaCircuit1x2x1v1x1Wasm": "mainSolanaCircuit1x2x1-1.1.wasm",
	"mainSolanaCircuit1x2x2v1x1Zkey": "mainSolanaCircuit1x2x2_final-1.1.zkey",
	"mainSolanaCircuit1x2x2v1x1Wasm": "mainSolanaCircuit1x2x2-1.1.wasm",
	"mainSolanaCircuit1x6x1v1x1Zkey": "mainSolanaCircuit1x6x1_final-1.1.zkey",
	"mainSolanaCircuit1x6x1v1x1Wasm": "mainSolanaCircuit1x6x1-1.1.wasm",
	"mainSolanaCircuit1x6x2v1x1Zkey": "mainSolanaCircuit1x6x2_final-1.1.zkey",
	"mainSolanaCircuit1x6x2v1x1Wasm": "mainSolanaCircuit1x6x2-1.1.wasm",
	"mainSolanaCircuit2x2x1v1x1Zkey": "mainSolanaCircuit2x2x1_final-1.1.zkey",
	"mainSolanaCircuit2x2x1v1x1Wasm": "mainSolanaCircuit2x2x1-1.1.wasm",
	"mainSolanaCircuit2x6x1v1x1Zkey": "mainSolanaCircuit2x6x1_final-1.1.zkey",
	"mainSolanaCircuit2x6x1v1x1Wasm": "mainSolanaCircuit2x6x1-1.1.wasm",

	"commitmentCalculator1x2v1x1Wasm": "commitmentCalculator1x2-1.1.wasm",
	"commitmentCalculator1x2v1x1Zkey": "commitmentCalculator1x2_final-1.1.zkey",
	"commitmentCalculator1x2v1x1VK":   "commitmentCalculator1x2_final-1.1_verification_key.json",
	"commitmentCalculator1x6v1x1Wasm": "commitmentCalculator1x6-1.1.wasm",
	"commitmentCalculator1x6v1x1Zkey": "commitmentCalculator1x6_final-1.1.zkey",
	"commitmentCalculator1x6v1x1VK":   "commitmentCalculator1x6_final-1.1_verification_key.json",
	"commitmentCalculator2x2v1x1Wasm": "commitmentCalculator2x2-1.1.wasm",
	"commitmentCalculator2x2v1x1Zkey": "commitmentCalculator2x2_final-1.1.zkey",
	"commitmentCalculator2x2v1x1VK":   "commitmentCalculator2x2_final-1.1_verification_key.json",
	"commitmentCalculator2x6v1x1Wasm": "commitmentCalculator2x6-1.1.wasm",
	"commitmentCalculator2x6v1x1Zkey": "commitmentCalculator2x6_final-1.1.zkey",
	"commitmentCalculator2x6v1x1VK":   "commitmentCalculator2x6_final-1.1_verification_key.json",
	"commitmentCalculator3x2v1x1Wasm": "commitmentCalculator3x2-1.1.wasm",
	"commitmentCalculator3x2v1x1Zkey": "commitmentCalculator3x2_final-1.1.zkey",
	"commitmentCalculator3x2v1x1VK":   "commitmentCalculator3x2_final-1.1_verification_key.json",
	"commitmentCalculator3x6v1x1Wasm": "commitmentCalculator3x6-1.1.wasm",
	"commitmentCalculator3x6v1x1Zkey": "commitmentCalculator3x6_final-1.1.zkey",
	"commitmentCalculator3x6v1x1VK":   "commitmentCalculator3x6_final-1.1_verification_key.json",
	"commitmentCalculator4x2v1x1Wasm": "commitmentCalculator4x2-1.1.wasm",
	"commitmentCalculator4x2v1x1Zkey": "commitmentCalculator4x2_final-1.1.zkey",
	"commitmentCalculator4x2v1x1VK":   "commitmentCalculator4x2_final-1.1_verification_key.json",
	"commitmentCalculator4x6v1x1Wasm": "commitmentCalculator4x6-1.1.wasm",
	"commitmentCalculator4x6v1x1Zkey": "commitmentCalculator4x6_final-1.1.zkey",
	"commitmentCalculator4x6v1x1VK":   "commitmentCalculator4x6_final-1.1_verification_key.json",
	"commitmentCalculator5x2v1x1Wasm": "commitmentCalculator5x2-1.1.wasm",
	"commitmentCalculator5x2v1x1Zkey": "commitmentCalculator5x2_final-1.1.zkey",
	"commitmentCalculator5x2v1x1VK":   "commitmentCalculator5x2_final-1.1_verification_key.json",
	"commitmentCalculator5x6v1x1Wasm": "commitmentCalculator5x6-1.1.wasm",
	"commitmentCalculator5x6v1x1Zkey": "commitmentCalculator5x6_final-1.1.zkey",
	"commitmentCalculator5x6v1x1VK":   "commitmentCalculator5x6_final-1.1_verification_key.json",
}

var prodArtifactNames = map[string]string{
	"mainEVMCircuit1x2x1v1x1Wasm": "mainEVMCircuit1x2x1-1.1.wasm",
	"mainEVMCircuit1x2x1v1x1Zkey": "mainEVMCircuit1x2x1_final-1.1.zkey",
	"mainEVMCircuit1x6x1v1x1Wasm": "mainEVMCircuit1x6x1-1.1.wasm",
	"mainEVMCircuit1x6x1v1x1Zkey": "mainEVMCircuit1x6x1_final-1.1.zkey",
	"mainEVMCircuit2x2x1v1x1Wasm": "mainEVMCircuit2x2x1-1.1.wasm",
	"mainEVMCircuit2x2x1v1x1Zkey": "mainEVMCircuit2x2x1_final-1.1.zkey",
	"mainEVMCircuit2x6x1v1x1Wasm": "mainEVMCircuit2x6x1-1.1.wasm",
	"mainEVMCircuit2x6x1v1x1Zkey": "mainEVMCircuit2x6x1_final-1.1.zkey",
	"mainEVMCircuit3x2x1v1x1Wasm": "mainEVMCircuit3x2x1-1.1.wasm",
	"mainEVMCircuit3x2x1v1x1Zkey": "mainEVMCircuit3x2x1_final-1.1.zkey",
	"mainEVMCircuit3x6x1v1x1Wasm": "mainEVMCircuit3x6x1-1.1.wasm",
	"mainEVMCircuit3x6x1v1x1Zkey": "mainEVMCircuit3x6x1_final-1.1.zkey",
	"mainEVMCircuit4x2x1v1x1Wasm": "mainEVMCircuit4x2x1-1.1.wasm",
	"mainEVMCircuit4x2x1v1x1Zkey": "mainEVMCircuit4x2x1_final-1.1.zkey",
	"mainEVMCircuit4x6x1v1x1Wasm": "mainEVMCircuit4x6x1-1.1.wasm",
	"mainEVMCircuit4x6x1v1x1Zkey": "mainEVMCircuit4x6x1_final-1.1.zkey",
	"mainEVMCircuit5x2x1v1x1Wasm": "mainEVMCircuit5x2x1-1.1.wasm",
	"mainEVMCircuit5x2x1v1x1Zkey": "mainEVMCircuit5x2x1_final-1.1.zkey",
	"mainEVMCircuit5x6x1v1x1Wasm": "mainEVMCircuit5x6x1-1.1.wasm",
	"mainEVMCircuit5x6x1v1x1Zkey": "mainEVMCircuit5x6x1_final-1.1.zkey",
	"mainEVMCircuit1x2x2v1x1Wasm": "mainEVMCircuit1x2x2-1.1.wasm",
	"mainEVMCircuit1x2x2v1x1Zkey": "mainEVMCircuit1x2x2_final-1.1.zkey",
	"mainEVMCircuit1x6x2v1x1Wasm": "mainEVMCircuit1x6x2-1.1.wasm",
	"mainEVMCircuit1x6x2v1x1Zkey": "mainEVMCircuit1x6x2_final-1.1.zkey",
	"mainEVMCircuit2x2x2v1x1Wasm": "mainEVMCircuit2x2x2-1.1.wasm",
	"mainEVMCircuit2x2x2v1x1Zkey": "mainEVMCircuit2x2x2_final-1.1.zkey",
	"mainEVMCircuit2x6x2v1x1Wasm": "mainEVMCircuit2x6x2-1.1.wasm",
	"mainEVMCircuit2x6x2v1x1Zkey": "mainEVMCircuit2x6x2_final-1.1.zkey",
	"mainEVMCircuitMin0v1x1Wasm":  "mainEVMCircuitMin0-1.1.wasm",
	"mainEVMCircuitMin0v1x1Zkey":  "mainEVMCircuitMin0_final-1.1.zkey",

	"mainSolanaCircuit1x2x1v1x1Zkey": "mainSolanaCircuit1x2x1_final-1.1.zkey",
	"mainSolanaCircuit1x2x1v1x1Wasm": "mainSolanaCircuit1x2x1-1.1.wasm",
	"mainSolanaCircuit1x2x2v1x1Zkey": "mainSolanaCircuit1x2x2_final-1.1.zkey",
	"mainSolanaCircuit1x2x2v1x1Wasm": "mainSolanaCircuit1x2x2-1.1.wasm",
	"mainSolanaCircuit1x6x1v1x1Zkey": "mainSolanaCircuit1x6x1_final-1.1.zkey",
	"mainSolanaCircuit1x6x1v1x1Wasm": "mainSolanaCircuit1x6x1-1.1.wasm",
	"mainSolanaCircuit1x6x2v1x1Zkey": "mainSolanaCircuit1x6x2_final-1.1.zkey",
	"mainSolanaCircuit1x6x2v1x1Wasm": "mainSolanaCircuit1x6x2-1.1.wasm",
	"mainSolanaCircuit2x2x1v1x1Zkey": "mainSolanaCircuit2x2x1_final-1.1.zkey",
	"mainSolanaCircuit2x2x1v1x1Wasm": "mainSolanaCircuit2x2x1-1.1.wasm",
	"mainSolanaCircuit2x6x1v1x1Zkey": "mainSolanaCircuit2x6x1_final-1.1.zkey",
	"mainSolanaCircuit2x6x1v1x1Wasm": "mainSolanaCircuit2x6x1-1.1.wasm",

	"commitmentCalculator1x2v1x1Wasm": "commitmentCalculator1x2-1.1.wasm",
	"commitmentCalculator1x2v1x1Zkey": "commitmentCalculator1x2_final-1.1.zkey",
	"commitmentCalculator1x2v1x1VK":   "commitmentCalculator1x2_final-1.1_verification_key.json",
	"commitmentCalculator1x6v1x1Wasm": "commitmentCalculator1x6-1.1.wasm",
	"commitmentCalculator1x6v1x1Zkey": "commitmentCalculator1x6_final-1.1.zkey",
	"commitmentCalculator1x6v1x1VK":   "commitmentCalculator1x6_final-1.1_verification_key.json",
	"commitmentCalculator2x2v1x1Wasm": "commitmentCalculator2x2-1.1.wasm",
	"commitmentCalculator2x2v1x1Zkey": "commitmentCalculator2x2_final-1.1.zkey",
	"commitmentCalculator2x2v1x1VK":   "commitmentCalculator2x2_final-1.1_verification_key.json",
	"commitmentCalculator2x6v1x1Wasm": "commitmentCalculator2x6-1.1.wasm",
	"commitmentCalculator2x6v1x1Zkey": "commitmentCalculator2x6_final-1.1.zkey",
	"commitmentCalculator2x6v1x1VK":   "commitmentCalculator2x6_final-1.1_verification_key.json",
	"commitmentCalculator3x2v1x1Wasm": "commitmentCalculator3x2-1.1.wasm",
	"commitmentCalculator3x2v1x1Zkey": "commitmentCalculator3x2_final-1.1.zkey",
	"commitmentCalculator3x2v1x1VK":   "commitmentCalculator3x2_final-1.1_verification_key.json",
	"commitmentCalculator3x6v1x1Wasm": "commitmentCalculator3x6-1.1.wasm",
	"commitmentCalculator3x6v1x1Zkey": "commitmentCalculator3x6_final-1.1.zkey",
	"commitmentCalculator3x6v1x1VK":   "commitmentCalculator3x6_final-1.1_verification_key.json",
	"commitmentCalculator4x2v1x1Wasm": "commitmentCalculator4x2-1.1.wasm",
	"commitmentCalculator4x2v1x1Zkey": "commitmentCalculator4x2_final-1.1.zkey",
	"commitmentCalculator4x2v1x1VK":   "commitmentCalculator4x2_final-1.1_verification_key.json",
	"commitmentCalculator4x6v1x1Wasm": "commitmentCalculator4x6-1.1.wasm",
	"commitmentCalculator4x6v1x1Zkey": "commitmentCalculator4x6_final-1.1.zkey",
	"commitmentCalculator4x6v1x1VK":   "commitmentCalculator4x6_final-1.1_verification_key.json",
	"commitmentCalculator5x2v1x1Wasm": "commitmentCalculator5x2-1.1.wasm",
	"commitmentCalculator5x2v1x1Zkey": "commitmentCalculator5x2_final-1.1.zkey",
	"commitmentCalculator5x2v1x1VK":   "commitmentCalculator5x2_final-1.1_verification_key.json",
	"commitmentCalculator5x6v1x1Wasm": "commitmentCalculator5x6-1.1.wasm",
	"commitmentCalculator5x6v1x1Zkey": "commitmentCalculator5x6_final-1.1.zkey",
	"commitmentCalculator5x6v1x1VK":   "commitmentCalculator5x6_final-1.1_verification_key.json",
}

var (
	prodVerifiersOnce sync.Once
	prodVerifiers     map[string]string
)

func getProdVerifiers() map[string]string {
	prodVerifiersOnce.Do(func() {
		gateway := constants.GetBackEndURL() + "/verifiers-v2/"
		prodVerifiers = make(map[string]string, len(prodArtifactNames))
		for key, name := range prodArtifactNames {
			prodVerifiers[key] = gateway + name
		}
	})
	return prodVerifiers
}

func isLocalProverChain(chainID int) bool {
	return chainID == constants.ChainIDs.Localhost ||
		chainID == constants.ChainIDs.SolanaLocalnet ||
		chainID == constants.ChainIDs.TronLocalnet
}

func GetWASMFile(filename string, chainID int) string {
	key := filename + ProverVersion + "Wasm"
	if isLocalProverChain(chainID) {
		return localVerifiers[key]
	}
	return getProdVerifiers()[key]
}

func GetZKeyFile(filename string, chainID int) string {
	key := filename + ProverVersion + "Zkey"
	if isLocalProverChain(chainID) {
		return localVerifiers[key]
	}
	return getProdVerifiers()[key]
}

func GetVKFile(filename string, chainID int) string {
	key := filename + ProverVersion + "VK"
	if isLocalProverChain(chainID) {
		return localVerifiers[key]
	}
	return getProdVerifiers()[key]
}
