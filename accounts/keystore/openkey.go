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

package keystore

import (
	"math/big"

	crand "crypto/rand"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
)

// EphemerealKeyStoreScheme is the protocol scheme prefixing account and wallet URLs.
const EphemerealKeyStoreScheme = "memory"

type OpenKeystore struct {
	keys     map[common.Address]*Key
	accounts []accounts.Account
}

func NewOpenKeystore() *OpenKeystore {
	return &OpenKeystore{
		keys:     make(map[common.Address]*Key),
		accounts: make([]accounts.Account, 0),
	}
}

func (o *OpenKeystore) URL() accounts.URL {
	return accounts.URL{
		Scheme: EphemerealKeyStoreScheme,
		Path:   "/dev/",
	}

}

func (o *OpenKeystore) NewAccount() (accounts.Account, error) {
	key, err := newKey(crand.Reader)
	if err != nil {
		return accounts.Account{}, err
	}
	a := accounts.Account{
		Address: key.Address,
		URL: accounts.URL{
			Scheme: EphemerealKeyStoreScheme,
			Path:   key.Address.Hex(),
		},
	}
	o.keys[key.Address] = key
	o.accounts = append(o.accounts, a)
	return a, err
}

func (o *OpenKeystore) Status() (string, error) {
	return "Unlocked", nil
}

func (o *OpenKeystore) Open(passphrase string) error { return nil }
func (o *OpenKeystore) Close() error                 { return nil }

func (o *OpenKeystore) Accounts() []accounts.Account {
	return o.accounts
}

func (o *OpenKeystore) Contains(account accounts.Account) bool {
	for _, acc := range o.accounts {
		if acc.Address == account.Address && (account.URL == (accounts.URL{}) || acc.URL == account.URL) {
			return true
		}
	}
	return false
}

func (o *OpenKeystore) Derive(path accounts.DerivationPath, pin bool) (accounts.Account, error) {
	return accounts.Account{}, accounts.ErrNotSupported
}

func (o *OpenKeystore) SelfDerive(bases []accounts.DerivationPath, chain ethereum.ChainStateReader) {}

func (o *OpenKeystore) SignData(account accounts.Account, mimeType string, data []byte) ([]byte, error) {
	return nil, accounts.ErrNotSupported
}

func (o *OpenKeystore) SignDataWithPassphrase(account accounts.Account, passphrase, mimeType string, data []byte) ([]byte, error) {
	return nil, accounts.ErrNotSupported
}

func (o *OpenKeystore) SignText(account accounts.Account, text []byte) ([]byte, error) {
	hash := accounts.TextHash(text)
	key, found := o.keys[account.Address]
	if !found {
		return nil, ErrNoMatch
	}
	return crypto.Sign(hash, key.PrivateKey)
}

func (o *OpenKeystore) SignTextWithPassphrase(account accounts.Account, passphrase string, hash []byte) ([]byte, error) {
	return nil, accounts.ErrNotSupported
}

func (o *OpenKeystore) SignTx(account accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	key, found := o.keys[account.Address]
	if !found {
		return nil, ErrNoMatch
	}
	signer := types.LatestSignerForChainID(chainID)
	return types.SignTx(tx, signer, key.PrivateKey)
}

func (o OpenKeystore) SignTxWithPassphrase(account accounts.Account, passphrase string, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	return nil, accounts.ErrNotSupported
}

type EphemBackend struct {
	ks *OpenKeystore
}

func NewEphemBackend(ks *OpenKeystore) *EphemBackend {
	return &EphemBackend{
		ks: ks,
	}

}
func (e *EphemBackend) Wallets() []accounts.Wallet {
	return []accounts.Wallet{e.ks}
}

func (e *EphemBackend) Subscribe(sink chan<- accounts.WalletEvent) event.Subscription {
	return event.NewSubscription(
		func(i <-chan struct{}) error {
			return nil
		})
}
