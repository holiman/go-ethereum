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
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/testdata/v2/nested_libraries"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"io"
	"math/big"
	"strings"
	"testing"
)

var testKey, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")

// JSON returns a parsed ABI interface and error if it failed.
func JSON(reader io.Reader) (abi.ABI, error) {
	dec := json.NewDecoder(reader)

	var instance abi.ABI
	if err := dec.Decode(&instance); err != nil {
		return abi.ABI{}, err
	}
	return instance, nil
}

func testSetup() (*bind.TransactOpts, *backends.SimulatedBackend, error) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	backend := simulated.NewBackend(
		types.GenesisAlloc{
			testAddr: {Balance: big.NewInt(10000000000000000)},
		},
		func(nodeConf *node.Config, ethConf *ethconfig.Config) {
			ethConf.Genesis.Difficulty = big.NewInt(0)
		},
	)

	_, err := JSON(strings.NewReader(nested_libraries.C1MetaData.ABI))
	if err != nil {
		return nil, nil, err
	}

	signer := types.LatestSigner(params.AllDevChainProtocolChanges)
	opts := bind.TransactOpts{
		From:  testAddr,
		Nonce: nil,
		Signer: func(address common.Address, tx *types.Transaction) (*types.Transaction, error) {
			signature, err := crypto.Sign(signer.Hash(tx).Bytes(), testKey)
			if err != nil {
				panic(fmt.Sprintf("error signing tx: %v", err))
				return nil, err
			}
			signedTx, err := tx.WithSignature(signer, signature)
			if err != nil {
				panic(fmt.Sprintf("error creating tx with sig: %v", err))
				return nil, err
			}
			return signedTx, nil
		},
		Context: context.Background(),
	}
	// we should just be able to use the backend directly, instead of using
	// this deprecated interface.  However, the simulated backend no longer
	// implements backends.SimulatedBackend...
	bindBackend := backends.SimulatedBackend{
		Backend: backend,
		Client:  backend.Client(),
	}
	return &opts, &bindBackend, nil
}

// test that deploying a contract with library dependencies works,
// verifying by calling the deployed contract.
func TestDeployment(t *testing.T) {
	opts, bindBackend, err := testSetup()
	if err != nil {
		t.Fatalf("err setting up test: %v", err)
	}
	defer bindBackend.Backend.Close()

	ctrct, err := nested_libraries.NewC1()
	if err != nil {
		panic(err)
	}

	constructorInput, err := ctrct.PackConstructor(big.NewInt(42), big.NewInt(1))
	if err != nil {
		t.Fatalf("failed to pack constructor: %v", err)
	}
	deploymentParams := DeploymentParams{
		Contracts: []ContractDeployParams{
			{
				Meta:  nested_libraries.C1MetaData,
				Input: constructorInput,
			},
		},
		Libraries: nested_libraries.C1LibraryDeps,
		Overrides: nil,
	}
	res, err := LinkAndDeploy(opts, bindBackend, deploymentParams)
	if err != nil {
		t.Fatalf("err: %+v\n", err)
	}
	bindBackend.Commit()

	if len(res.Addrs) != 5 {
		t.Fatalf("deployment should have generated 5 addresses.  got %d", len(res.Addrs))
	}
	for _, tx := range res.Txs {
		_, err = bind.WaitDeployed(context.Background(), bindBackend, tx)
		if err != nil {
			t.Fatalf("error deploying library: %+v", err)
		}
	}
	c, err := nested_libraries.NewC1()
	if err != nil {
		t.Fatalf("err is %v", err)
	}
	doInput, err := c.PackDo(big.NewInt(1))
	if err != nil {
		t.Fatalf("pack function input err: %v\n", doInput)
	}

	cABI, err := nested_libraries.C1MetaData.GetAbi()
	if err != nil {
		t.Fatalf("error getting abi object: %v", err)
	}
	contractAddr := res.Addrs[nested_libraries.C1MetaData.Pattern]
	boundC := bind.NewBoundContract(contractAddr, *cABI, bindBackend, bindBackend, bindBackend)
	callOpts := &bind.CallOpts{
		From:    common.Address{},
		Context: context.Background(),
	}
	callRes, err := boundC.CallRaw(callOpts, doInput)
	if err != nil {
		t.Fatalf("err calling contract: %v", err)
	}
	internalCallCount, err := c.UnpackDo(callRes)
	if err != nil {
		t.Fatalf("err unpacking result: %v", err)
	}
	if internalCallCount.Uint64() != 6 {
		t.Fatalf("expected internal call count of 6.  got %d.", internalCallCount.Uint64())
	}
}

func TestDeploymentWithOverrides(t *testing.T) {
	opts, bindBackend, err := testSetup()
	if err != nil {
		t.Fatalf("err setting up test: %v", err)
	}
	defer bindBackend.Backend.Close()

	// deploy some library deps
	deploymentParams := DeploymentParams{
		Libraries: nested_libraries.C1LibraryDeps,
	}

	res, err := LinkAndDeploy(opts, bindBackend, deploymentParams)
	if err != nil {
		t.Fatalf("err: %+v\n", err)
	}
	bindBackend.Commit()

	if len(res.Addrs) != 4 {
		t.Fatalf("deployment should have generated 4 addresses.  got %d", len(res.Addrs))
	}
	for _, tx := range res.Txs {
		_, err = bind.WaitDeployed(context.Background(), bindBackend, tx)
		if err != nil {
			t.Fatalf("error deploying library: %+v", err)
		}
	}

	ctrct, err := nested_libraries.NewC1()
	if err != nil {
		panic(err)
	}
	constructorInput, err := ctrct.PackConstructor(big.NewInt(42), big.NewInt(1))
	if err != nil {
		t.Fatalf("failed to pack constructor: %v", err)
	}
	overrides := res.Addrs
	// deploy the contract
	deploymentParams = DeploymentParams{
		Contracts: []ContractDeployParams{
			{
				Meta:  nested_libraries.C1MetaData,
				Input: constructorInput,
			},
		},
		Libraries: nil,
		Overrides: overrides,
	}
	res, err = LinkAndDeploy(opts, bindBackend, deploymentParams)
	if err != nil {
		t.Fatalf("err: %+v\n", err)
	}
	bindBackend.Commit()

	if len(res.Addrs) != 1 {
		t.Fatalf("deployment should have generated 1 address.  got %d", len(res.Addrs))
	}
	for _, tx := range res.Txs {
		_, err = bind.WaitDeployed(context.Background(), bindBackend, tx)
		if err != nil {
			t.Fatalf("error deploying library: %+v", err)
		}
	}

	// call the deployed contract and make sure it returns the correct result
	c, err := nested_libraries.NewC1()
	if err != nil {
		t.Fatalf("err is %v", err)
	}
	doInput, err := c.PackDo(big.NewInt(1))
	if err != nil {
		t.Fatalf("pack function input err: %v\n", doInput)
	}

	cABI, err := nested_libraries.C1MetaData.GetAbi()
	if err != nil {
		t.Fatalf("error getting abi object: %v", err)
	}
	contractAddr := res.Addrs[nested_libraries.C1MetaData.Pattern]
	boundC := bind.NewBoundContract(contractAddr, *cABI, bindBackend, bindBackend, bindBackend)
	callOpts := &bind.CallOpts{
		From:    common.Address{},
		Context: context.Background(),
	}
	callRes, err := boundC.CallRaw(callOpts, doInput)
	if err != nil {
		t.Fatalf("err calling contract: %v", err)
	}
	internalCallCount, err := c.UnpackDo(callRes)
	if err != nil {
		t.Fatalf("err unpacking result: %v", err)
	}
	if internalCallCount.Uint64() != 6 {
		t.Fatalf("expected internal call count of 6.  got %d.", internalCallCount.Uint64())
	}
}

/*
 *
 */
/*
	func TestDeploymentWithOverrides(t *testing.T) {
		// more deployment test case ideas:
		// 1)  deploy libraries, then deploy contract first with libraries as overrides
		// 2)  deploy contract without library dependencies.
	}
*/

func TestEvents(t *testing.T) {
	// test watch/filter logs method on a contract that emits various kinds of events (struct-containing, etc.)
}
