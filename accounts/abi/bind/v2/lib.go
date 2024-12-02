// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package v2

import (
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"regexp"
	"strings"
)

// deployContract deploys a hex-encoded contract with the given constructor
// input.  It returns the deployment transaction, address on success.
func deployContract(backend bind.ContractBackend, auth *bind.TransactOpts, constructor []byte, contract string) (deploymentTx *types.Transaction, deploymentAddr common.Address, err error) {
	contractBinBytes, err := hex.DecodeString(contract[2:])
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("contract bytecode is not a hex string: %s", contractBinBytes[2:])
	}
	addr, tx, _, err := bind.DeployContractRaw(auth, contractBinBytes, backend, constructor)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to deploy contract: %v", err)
	}
	return tx, addr, nil
}

// deployLibs iterates the set contracts (map of pattern to hex-encoded
// contract deployer code). Each contract is deployed, and the
// resulting addresses/deployment-txs are returned on success.
func deployLibs(backend bind.ContractBackend, auth *bind.TransactOpts, contracts map[string]string) (deploymentTxs map[common.Address]*types.Transaction, deployAddrs map[string]common.Address, err error) {
	deploymentTxs = make(map[common.Address]*types.Transaction)
	deployAddrs = make(map[string]common.Address)

	for pattern, contractBin := range contracts {
		contractBinBytes, err := hex.DecodeString(contractBin[2:])
		if err != nil {
			return deploymentTxs, deployAddrs, fmt.Errorf("contract bytecode is not a hex string: %s", contractBin[2:])
		}
		// TODO: can pass nil for constructor?
		addr, tx, _, err := bind.DeployContractRaw(auth, contractBinBytes, backend, []byte{})
		if err != nil {
			return deploymentTxs, deployAddrs, fmt.Errorf("failed to deploy contract: %v", err)
		}
		deploymentTxs[addr] = tx
		deployAddrs[pattern] = addr
	}

	return deploymentTxs, deployAddrs, nil
}

// linkContract takes an unlinked contract deployer hex-encoded code, a map of
// already-deployed library dependencies, replaces references to deployed library
// dependencies in the contract code, and returns the contract deployment bytecode on
// success.
func linkContract(contract string, linkedLibs map[string]common.Address) (deployableContract string, err error) {
	reMatchSpecificPattern, err := regexp.Compile("__\\$([a-f0-9]+)\\$__")
	if err != nil {
		return "", err
	}
	// link in any library the contract depends on
	for _, match := range reMatchSpecificPattern.FindAllStringSubmatch(contract, -1) {
		matchingPattern := match[1]
		addr := linkedLibs[matchingPattern]
		contract = strings.ReplaceAll(contract, "__$"+matchingPattern+"$__", addr.String()[2:])
	}
	return contract, nil
}

// linkLibs iterates the set of dependencies that have yet to be
// linked/deployed (pending), replacing references to library dependencies
// (i.e. mutating pending) if those dependencies are fully linked/deployed
// (in 'linked').
//
// contracts that have become fully linked in the current invocation are
// returned.
func linkLibs(pending *map[string]string, linked map[string]common.Address) (deployableDeps map[string]string) {
	reMatchSpecificPattern, err := regexp.Compile("__\\$([a-f0-9]+)\\$__")
	if err != nil {
		panic(err)
	}
	reMatchAnyPattern, err := regexp.Compile("__\\$.*\\$__")
	if err != nil {
		panic(err)
	}
	deployableDeps = make(map[string]string)

	for pattern, dep := range *pending {
		// link references to dependent libraries that have been deployed
		for _, match := range reMatchSpecificPattern.FindAllStringSubmatch(dep, -1) {
			matchingPattern := match[1]
			addr, ok := linked[matchingPattern]
			if !ok {
				continue
			}
			(*pending)[pattern] = strings.ReplaceAll(dep, "__$"+matchingPattern+"$__", addr.String()[2:])
		}
		// if the library code became fully linked, move it from pending->linked.
		if !reMatchAnyPattern.MatchString((*pending)[pattern]) {
			deployableDeps[pattern] = (*pending)[pattern]
			delete(*pending, pattern)
		}
	}
	return deployableDeps
}

// ContractDeployParams represents state needed to deploy a contract:
// the metdata and constructor input (which can be nil if no input is specified).
type ContractDeployParams struct {
	Meta *bind.MetaData
	// Input is the ABI-encoded constructor input for the contract deployment.
	Input []byte
}

// DeploymentParams represents parameters needed to deploy a
// set of contracts, their dependency libraries.  It takes an optional override
// list to specify libraries that have already been deployed on-chain.
type DeploymentParams struct {
	// Contracts is the set of contract deployment parameters for contracts
	// that are about to be deployed.
	Contracts []ContractDeployParams
	// Libraries is a map of pattern to metadata for library contracts that
	// are to be deployed.
	Libraries []*bind.MetaData
	// Overrides is an optional map of pattern to deployment address.
	// Contracts/libraries that refer to dependencies in the override
	// set are linked to the provided address (an already-deployed contract).
	Overrides map[string]common.Address
}

// DeploymentResult contains the relevant information from the deployment of
// multiple contracts:  their deployment txs and addresses.
type DeploymentResult struct {
	// map of contract library pattern -> deploy transaction
	Txs map[string]*types.Transaction
	// map of contract library pattern -> deployed address
	Addrs map[string]common.Address
}

// LinkAndDeploy deploys a specified set of contracts and their dependent
// libraries.  If an error occurs, only contracts which were successfully
// deployed are returned in the result.
func LinkAndDeploy(auth *bind.TransactOpts, backend bind.ContractBackend, deployParams DeploymentParams) (res *DeploymentResult, err error) {
	libMetas := deployParams.Libraries
	overrides := deployParams.Overrides

	res = &DeploymentResult{
		Txs:   make(map[string]*types.Transaction),
		Addrs: make(map[string]common.Address),
	}

	// re-express libraries as a map of pattern -> pre-link binary
	pending := make(map[string]string)
	for _, meta := range libMetas {
		pending[meta.Pattern] = meta.Bin
	}

	// initialize the set of already-deployed contracts with given override addresses
	deployed := make(map[string]common.Address)
	for pattern, deployAddr := range overrides {
		deployed[pattern] = deployAddr
		if _, ok := pending[pattern]; ok {
			delete(pending, pattern)
		}
	}

	// link and deploy dynamic libraries
	for {
		deployableDeps := linkLibs(&pending, deployed)
		if len(deployableDeps) == 0 {
			break
		}
		deployTxs, deployAddrs, err := deployLibs(backend, auth, deployableDeps)
		for pattern, addr := range deployAddrs {
			deployed[pattern] = addr
			res.Addrs[pattern] = addr
			res.Txs[pattern] = deployTxs[addr]
		}
		if err != nil {
			return res, err
		}
	}

	// link and deploy contracts
	for _, contractParams := range deployParams.Contracts {
		linkedContract, err := linkContract(contractParams.Meta.Bin, deployed)
		if err != nil {
			return res, err
		}
		contractTx, contractAddr, err := deployContract(backend, auth, contractParams.Input, linkedContract)
		if err != nil {
			return res, err
		}
		res.Txs[contractParams.Meta.Pattern] = contractTx
		res.Addrs[contractParams.Meta.Pattern] = contractAddr
	}

	return res, nil
}
