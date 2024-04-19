package state

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

type journal interface {

	// snapshot returns an identifier for the current revision of the state.
	snapshot() int

	// revertToSnapshot reverts all state changes made since the given revision.
	revertToSnapshot(revid int, s *StateDB)

	// reset clears the journal so it can be reused.
	reset()

	// dirtyAccounts returns a list of all accounts modified in this journal
	dirtyAccounts() []common.Address

	// accessListAddAccount journals the adding of addr to the access list
	accessListAddAccount(addr common.Address)

	// accessListAddSlot journals the adding of addr/slot to the access list
	accessListAddSlot(addr common.Address, slot common.Hash)

	// logChange journals the adding of a log related to the txHash
	logChange(txHash common.Hash)

	// createObject journals the event of a new account created in the trie.
	createObject(addr common.Address)

	// createContract journals the creation of a new contract at addr.
	// OBS: This method must not be applied twice, it assumes that the pre-state
	// (i.e the rollback-state) is non-created.
	createContract(addr common.Address)

	// destruct journals the destruction of an account in the trie.
	// OBS: This method must not be applied twice -- it always assumes that the
	// pre-state (i.e the rollback-state) is non-destructed.
	destruct(addr common.Address)

	// storageChange journals a change in the storage data related to addr.
	// It records the key and previous value of the slot.
	storageChange(addr common.Address, key, prev, origin common.Hash)

	// transientStateChange journals a change in the t-storage data related to addr.
	// It records the key and previous value of the slot.
	transientStateChange(addr common.Address, key, prev common.Hash)

	// refundChange journals that the refund has been changed, recording the previous value.
	refundChange(previous uint64)

	// balanceChange journals tha the balance of addr has been changed, recording the previous value
	balanceChange(addr common.Address, previous *uint256.Int)

	// JournalSetCode journals that the code of addr has been set.
	// OBS: This method must not be applied twice -- it always assumes that the
	// pre-state (i.e the rollback-state) is "no code".
	setCode(addr common.Address)

	// nonceChange journals that the nonce of addr was changed, recording the previous value.
	nonceChange(addr common.Address, prev uint64)

	// touchChange journals that the account at addr was touched during execution.
	touchChange(addr common.Address)

	// copy returns a deep-copied journal.
	copy() journal
}
