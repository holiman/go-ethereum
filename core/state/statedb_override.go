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

package state

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie/utils"
	"github.com/holiman/uint256"
)

// overrideState represents a statedb which contains user-specified overrides
type overrideState struct {
	inner     StateDatabase
	overrides StateOverride
}

// NewOverrideState wraps the given stateDb with the given overrides.
func NewOverrideState(stateDb StateDatabase, overrides StateOverride) *overrideState {
	return &overrideState{stateDb, overrides}
}

func (s *overrideState) CreateAccount(addr common.Address) {
	s.inner.CreateAccount(addr)
}

func (s *overrideState) CreateContract(addr common.Address) {
	s.inner.CreateContract(addr)
}

func (s *overrideState) GetBalance(addr common.Address) *uint256.Int {
	if acc, ok := s.overrides[addr]; ok {
		if acc.Balance != nil {
			return acc.Balance
		}
	}
	return s.inner.GetBalance(addr)
}

func (s *overrideState) GetNonce(addr common.Address) uint64 {
	if acc, ok := s.overrides[addr]; ok {
		if acc.Nonce != nil {
			return *acc.Nonce
		}
	}
	return s.inner.GetNonce(addr)
}

func (s *overrideState) GetCodeHash(addr common.Address) common.Hash {
	if acc, ok := s.overrides[addr]; ok {
		if acc.Code != nil {
			return crypto.Keccak256Hash(*acc.Code)
		}
	}
	return s.inner.GetCodeHash(addr)
}

func (s *overrideState) GetCode(addr common.Address) []byte {
	if acc, ok := s.overrides[addr]; ok {
		if acc.Code != nil {
			return *acc.Code
		}
	}
	return s.inner.GetCode(addr)
}

func (s *overrideState) GetCodeSize(addr common.Address) int {
	return s.inner.GetCodeSize(addr)
}

func (s *overrideState) AddRefund(u uint64) {
	s.inner.AddRefund(u)
}

func (s *overrideState) SubRefund(u uint64) {
	s.inner.SubRefund(u)
}

func (s *overrideState) GetRefund() uint64 {
	return s.inner.GetRefund()
}

func (s *overrideState) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	return s.inner.GetCommittedState(addr, hash)
}

func (s *overrideState) GetState(addr common.Address, hash common.Hash) common.Hash {
	acc, ok := s.overrides[addr]
	if !ok {
		return s.inner.GetState(addr, hash)
	}
	if acc.State != nil {
		// If State is present, then it overrides all storage queries
		return acc.State[hash]
	} else if acc.StateDiff != nil {
		// StateDiff is only set for some select slots, otherwise go to inner
		if v, ok := acc.StateDiff[hash]; ok {
			return v
		}
	}
	return s.inner.GetState(addr, hash)
}

func (s *overrideState) GetStorageRoot(addr common.Address) common.Hash {
	return s.inner.GetStorageRoot(addr)
}

func (s *overrideState) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return s.inner.GetTransientState(addr, key)
}

func (s *overrideState) SetTransientState(addr common.Address, key, value common.Hash) {
	s.inner.SetTransientState(addr, key, value)
}

func (s *overrideState) HasSelfDestructed(addr common.Address) bool {
	return s.inner.HasSelfDestructed(addr)
}

func (s *overrideState) Exist(addr common.Address) bool {
	return s.inner.Exist(addr)
}

func (s *overrideState) Empty(addr common.Address) bool {
	return s.inner.Empty(addr)
}

func (s *overrideState) AddressInAccessList(addr common.Address) bool {
	return s.inner.AddressInAccessList(addr)
}

func (s *overrideState) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return s.inner.SlotInAccessList(addr, slot)
}

func (s *overrideState) AddAddressToAccessList(addr common.Address) {
	s.inner.AddAddressToAccessList(addr)
}

func (s *overrideState) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	s.inner.AddSlotToAccessList(addr, slot)
}

func (s *overrideState) PointCache() *utils.PointCache {
	return s.inner.PointCache()
}

func (s *overrideState) Prepare(rules params.Rules, sender, coinbase common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	s.inner.Prepare(rules, sender, coinbase, dest, precompiles, txAccesses)
}

func (s *overrideState) RevertToSnapshot(i int) {
	s.inner.RevertToSnapshot(i)
}

func (s *overrideState) Snapshot() int {
	return s.inner.Snapshot()
}

func (s *overrideState) AddPreimage(hash common.Hash, bytes []byte) {
	s.inner.AddPreimage(hash, bytes)
}

func (s *overrideState) Witness() *stateless.Witness {
	return s.inner.Witness()
}

func (s *overrideState) SubBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	if acc, ok := s.overrides[addr]; ok {
		if acc.Balance != nil {
			prev := acc.Balance.Clone()
			acc.Balance.Sub(acc.Balance, amount)
			return *prev
		}
	}
	return s.inner.SubBalance(addr, amount, reason)
}

func (s *overrideState) AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	if acc, ok := s.overrides[addr]; ok {
		if acc.Balance != nil {
			prev := acc.Balance.Clone()
			acc.Balance.Add(acc.Balance, amount)
			return *prev
		}
	}
	return s.inner.AddBalance(addr, amount, reason)
}

func (s *overrideState) SetNonce(addr common.Address, nonce uint64) {
	if acc, ok := s.overrides[addr]; ok {
		acc.Nonce = &nonce
		return
	}
	s.inner.SetNonce(addr, nonce)
}

func (s *overrideState) SetCode(addr common.Address, code []byte) {
	if acc, ok := s.overrides[addr]; ok {
		acc.Code = &code // Do we need to copy first?
		return
	}
	s.inner.SetCode(addr, code)
}

func (s *overrideState) SetState(addr common.Address, key common.Hash, value common.Hash) common.Hash {
	if acc, ok := s.overrides[addr]; ok {
		if acc.State != nil { // If State is present, then it overrides all storage queries
			prev := acc.State[key]
			acc.State[key] = value
			return prev
		}
		if acc.StateDiff != nil { // StateDiff is only set for some select slots
			if prev, ok := acc.StateDiff[key]; ok {
				acc.StateDiff[key] = value
				return prev
			}
		}
	}
	return s.inner.SetState(addr, key, value)
}

func (s *overrideState) SelfDestruct(address common.Address) uint256.Int {
	return s.inner.SelfDestruct(address)
}

func (s *overrideState) SelfDestruct6780(address common.Address) (uint256.Int, bool) {
	return s.inner.SelfDestruct6780(address)
}

func (s *overrideState) AddLog(log *types.Log) {
	s.inner.AddLog(log)
}

func (s *overrideState) Finalise(deleteEmptyObjects bool) {
	s.inner.Finalise(deleteEmptyObjects)
}
