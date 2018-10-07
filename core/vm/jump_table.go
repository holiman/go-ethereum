// Copyright 2015 The go-ethereum Authors
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

package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/params"
)

type executionContext struct {
	pc          *uint64
	interpreter *EVMInterpreter
	contract    *Contract
	memory      *Memory
	stack       *Stack
}

type (
	executionFunc func(*executionContext) ([]byte, error)
	//executionFunc       func(pc *uint64, interpreter *EVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error)
	gasFunc             func(params.GasTable, *EVM, *Contract, *Stack, *Memory, uint64) (uint64, error) // last parameter is the requested memory size as a uint64
	stackValidationFunc func(*Stack) error
	memorySizeFunc      func(*Stack) *big.Int
)

var errGasUintOverflow = errors.New("gas uint64 overflow")

type operation struct {
	// execute is the operation function
	execute executionFunc
	// validateStack validates the stack (size) for the operation
	validateStack stackValidationFunc
	// memorySize returns the memory size required for the operation
	memorySize memorySizeFunc

	halts      bool    // indicates whether the operation should halt further execution
	jumps      bool    // indicates whether the program counter should not increment
	writes     bool    // determines whether this a state modifying operation
	valid      bool    // indication whether the retrieved operation is valid and known
	reverts    bool    // determines whether the operation reverts state (implicitly halts)
	returns    bool    // determines whether the operations sets the return data content
	constGas   uint64  // The static cost of the operation
	dynamicGas gasFunc // The dynamic part of the gascost for the operation
}

var (
	frontierInstructionSet       = newFrontierInstructionSet()
	homesteadInstructionSet      = newHomesteadInstructionSet()
	byzantiumInstructionSet      = newByzantiumInstructionSet()
	constantinopleInstructionSet = newConstantinopleInstructionSet()
)

// NewConstantinopleInstructionSet returns the frontier, homestead
// byzantium and contantinople instructions.
func newConstantinopleInstructionSet() [256]operation {
	// instructions that can be executed during the byzantium phase.
	instructionSet := newByzantiumInstructionSet()
	instructionSet[SHL] = operation{
		execute:       opSHL,
		constGas:      GasFastestStep,
		validateStack: makeStackFunc(2, 1),
		valid:         true,
	}
	instructionSet[SHR] = operation{
		execute:       opSHR,
		constGas:      GasFastestStep,
		validateStack: makeStackFunc(2, 1),
		valid:         true,
	}
	instructionSet[SAR] = operation{
		execute:       opSAR,
		constGas:      GasFastestStep,
		validateStack: makeStackFunc(2, 1),
		valid:         true,
	}
	instructionSet[EXTCODEHASH] = operation{
		execute:       opExtCodeHash,
		dynamicGas:    gasExtCodeHash,
		validateStack: makeStackFunc(1, 1),
		valid:         true,
	}
	instructionSet[CREATE2] = operation{
		execute:       opCreate2,
		dynamicGas:    gasCreate2,
		validateStack: makeStackFunc(4, 1),
		memorySize:    memoryCreate2,
		valid:         true,
		writes:        true,
		returns:       true,
	}
	return instructionSet
}

// NewByzantiumInstructionSet returns the frontier, homestead and
// byzantium instructions.
func newByzantiumInstructionSet() [256]operation {
	// instructions that can be executed during the homestead phase.
	instructionSet := newHomesteadInstructionSet()
	instructionSet[STATICCALL] = operation{
		execute:       opStaticCall,
		dynamicGas:    gasStaticCall,
		validateStack: makeStackFunc(6, 1),
		memorySize:    memoryStaticCall,
		valid:         true,
		returns:       true,
	}
	instructionSet[RETURNDATASIZE] = operation{
		execute:       opReturnDataSize,
		constGas:      GasQuickStep,
		validateStack: makeStackFunc(0, 1),
		valid:         true,
	}
	instructionSet[RETURNDATACOPY] = operation{
		execute:       opReturnDataCopy,
		dynamicGas:    gasReturnDataCopy,
		validateStack: makeStackFunc(3, 0),
		memorySize:    memoryReturnDataCopy,
		valid:         true,
	}
	instructionSet[REVERT] = operation{
		execute:       opRevert,
		dynamicGas:    gasRevert,
		validateStack: makeStackFunc(2, 0),
		memorySize:    memoryRevert,
		valid:         true,
		reverts:       true,
		returns:       true,
	}
	return instructionSet
}

// NewHomesteadInstructionSet returns the frontier and homestead
// instructions that can be executed during the homestead phase.
func newHomesteadInstructionSet() [256]operation {
	instructionSet := newFrontierInstructionSet()
	instructionSet[DELEGATECALL] = operation{
		execute:       opDelegateCall,
		dynamicGas:    gasDelegateCall,
		validateStack: makeStackFunc(6, 1),
		memorySize:    memoryDelegateCall,
		valid:         true,
		returns:       true,
	}
	return instructionSet
}

// NewFrontierInstructionSet returns the frontier instructions
// that can be executed during the frontier phase.
func newFrontierInstructionSet() [256]operation {
	return [256]operation{
		STOP: {
			execute:       opStop,
			constGas:      0,
			validateStack: makeStackFunc(0, 0),
			halts:         true,
			valid:         true,
		},
		ADD: {
			execute:       opAdd,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		MUL: {
			execute:       opMul,
			constGas:      GasFastStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SUB: {
			execute:       opSub,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		DIV: {
			execute:       opDiv,
			constGas:      GasFastStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SDIV: {
			execute:       opSdiv,
			constGas:      GasFastStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		MOD: {
			execute:       opMod,
			constGas:      GasFastStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SMOD: {
			execute:       opSmod,
			constGas:      GasFastStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		ADDMOD: {
			execute:       opAddmod,
			constGas:      GasMidStep,
			validateStack: makeStackFunc(3, 1),
			valid:         true,
		},
		MULMOD: {
			execute:       opMulmod,
			constGas:      GasMidStep,
			validateStack: makeStackFunc(3, 1),
			valid:         true,
		},
		EXP: {
			execute:       opExp,
			dynamicGas:    gasExp,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SIGNEXTEND: {
			execute:       opSignExtend,
			constGas:      GasFastStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		LT: {
			execute:       opLt,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		GT: {
			execute:       opGt,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SLT: {
			execute:       opSlt,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SGT: {
			execute:       opSgt,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		EQ: {
			execute:       opEq,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		ISZERO: {
			execute:       opIszero,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		AND: {
			execute:       opAnd,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		XOR: {
			execute:       opXor,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		OR: {
			execute:       opOr,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		NOT: {
			execute:       opNot,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		BYTE: {
			execute:       opByte,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(2, 1),
			valid:         true,
		},
		SHA3: {
			execute:       opSha3,
			dynamicGas:    gasSha3,
			validateStack: makeStackFunc(2, 1),
			memorySize:    memorySha3,
			valid:         true,
		},
		ADDRESS: {
			execute:       opAddress,
			constGas:      GasQuickStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		BALANCE: {
			execute:       opBalance,
			dynamicGas:    gasBalance,
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		ORIGIN: {
			execute:       opOrigin,
			constGas:      GasQuickStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		CALLER: {
			execute:       opCaller,
			constGas:      GasQuickStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		CALLVALUE: {
			execute:       opCallValue,
			constGas:      GasQuickStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		CALLDATALOAD: {
			execute:       opCallDataLoad,
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		CALLDATASIZE: {
			execute:       opCallDataSize,
			constGas:      GasQuickStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		CALLDATACOPY: {
			execute:       opCallDataCopy,
			dynamicGas:    gasCallDataCopy,
			validateStack: makeStackFunc(3, 0),
			memorySize:    memoryCallDataCopy,
			valid:         true,
		},
		CODESIZE: {
			execute:       opCodeSize,
			constGas:      GasQuickStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		CODECOPY: {
			execute:       opCodeCopy,
			dynamicGas:    gasCodeCopy,
			validateStack: makeStackFunc(3, 0),
			memorySize:    memoryCodeCopy,
			valid:         true,
		},
		GASPRICE: {
			execute:       opGasprice,
			constGas:      GasQuickStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		EXTCODESIZE: {
			execute:       opExtCodeSize,
			dynamicGas:    gasExtCodeSize,
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		EXTCODECOPY: {
			execute:       opExtCodeCopy,
			dynamicGas:    gasExtCodeCopy,
			validateStack: makeStackFunc(4, 0),
			memorySize:    memoryExtCodeCopy,
			valid:         true,
		},
		BLOCKHASH: {
			execute:       opBlockhash,
			constGas:      GasExtStep,
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		COINBASE: {
			execute:       opCoinbase,
			constGas:      GasQuickStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		TIMESTAMP: {
			execute:       opTimestamp,
			constGas:      (GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		NUMBER: {
			execute:       opNumber,
			constGas:      (GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		DIFFICULTY: {
			execute:       opDifficulty,
			constGas:      (GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		GASLIMIT: {
			execute:       opGasLimit,
			constGas:      (GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		POP: {
			execute:       opPop,
			constGas:      (GasQuickStep),
			validateStack: makeStackFunc(1, 0),
			valid:         true,
		},
		MLOAD: {
			execute:       opMload,
			dynamicGas:    gasMLoad,
			validateStack: makeStackFunc(1, 1),
			memorySize:    memoryMLoad,
			valid:         true,
		},
		MSTORE: {
			execute:       opMstore,
			dynamicGas:    gasMStore,
			validateStack: makeStackFunc(2, 0),
			memorySize:    memoryMStore,
			valid:         true,
		},
		MSTORE8: {
			execute:       opMstore8,
			dynamicGas:    gasMStore8,
			memorySize:    memoryMStore8,
			validateStack: makeStackFunc(2, 0),

			valid: true,
		},
		SLOAD: {
			execute:       opSload,
			dynamicGas:    gasSLoad,
			validateStack: makeStackFunc(1, 1),
			valid:         true,
		},
		SSTORE: {
			execute:       opSstore,
			dynamicGas:    gasSStore,
			validateStack: makeStackFunc(2, 0),
			valid:         true,
			writes:        true,
		},
		JUMP: {
			execute:       opJump,
			constGas:      GasMidStep,
			validateStack: pop1push0,
			jumps:         true,
			valid:         true,
		},
		JUMPI: {
			execute:       opJumpi,
			constGas:      (GasSlowStep),
			validateStack: makeStackFunc(2, 0),
			jumps:         true,
			valid:         true,
		},
		PC: {
			execute:       opPc,
			constGas:      (GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		MSIZE: {
			execute:       opMsize,
			constGas:      (GasQuickStep),
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		GAS: {
			execute:       opGas,
			constGas:      GasQuickStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		JUMPDEST: {
			execute:       opJumpdest,
			constGas:      params.JumpdestGas,
			validateStack: pop0push0,
			valid:         true,
		},
		PUSH1: {
			execute:       push1,
			constGas:      GasFastestStep,
			validateStack: pop0push1,
			valid:         true,
		},
		PUSH2: {
			execute:       makePush(2, 2),
			constGas:      GasFastestStep,
			validateStack: pop0push1,
			valid:         true,
		},
		PUSH3: {
			execute:       makePush(3, 3),
			constGas:      GasFastestStep,
			validateStack: pop0push1,
			valid:         true,
		},
		PUSH4: {
			execute:       makePush(4, 4),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH5: {
			execute:       makePush(5, 5),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH6: {
			execute:       makePush(6, 6),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH7: {
			execute:       makePush(7, 7),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH8: {
			execute:       makePush(8, 8),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH9: {
			execute:       makePush(9, 9),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH10: {
			execute:       makePush(10, 10),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH11: {
			execute:       makePush(11, 11),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH12: {
			execute:       makePush(12, 12),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH13: {
			execute:       makePush(13, 13),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH14: {
			execute:       makePush(14, 14),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH15: {
			execute:       makePush(15, 15),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH16: {
			execute:       makePush(16, 16),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH17: {
			execute:       makePush(17, 17),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH18: {
			execute:       makePush(18, 18),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH19: {
			execute:       makePush(19, 19),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH20: {
			execute:       makePush(20, 20),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH21: {
			execute:       makePush(21, 21),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH22: {
			execute:       makePush(22, 22),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH23: {
			execute:       makePush(23, 23),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH24: {
			execute:       makePush(24, 24),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH25: {
			execute:       makePush(25, 25),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH26: {
			execute:       makePush(26, 26),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH27: {
			execute:       makePush(27, 27),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH28: {
			execute:       makePush(28, 28),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH29: {
			execute:       makePush(29, 29),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH30: {
			execute:       makePush(30, 30),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH31: {
			execute:       makePush(31, 31),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		PUSH32: {
			execute:       makePush(32, 32),
			constGas:      GasFastestStep,
			validateStack: makeStackFunc(0, 1),
			valid:         true,
		},
		DUP1: {
			execute:       makeDup(1),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(1),
			valid:         true,
		},
		DUP2: {
			execute:       makeDup(2),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(2),
			valid:         true,
		},
		DUP3: {
			execute:       makeDup(3),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(3),
			valid:         true,
		},
		DUP4: {
			execute:       makeDup(4),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(4),
			valid:         true,
		},
		DUP5: {
			execute:       makeDup(5),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(5),
			valid:         true,
		},
		DUP6: {
			execute:       makeDup(6),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(6),
			valid:         true,
		},
		DUP7: {
			execute:       makeDup(7),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(7),
			valid:         true,
		},
		DUP8: {
			execute:       makeDup(8),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(8),
			valid:         true,
		},
		DUP9: {
			execute:       makeDup(9),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(9),
			valid:         true,
		},
		DUP10: {
			execute:       makeDup(10),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(10),
			valid:         true,
		},
		DUP11: {
			execute:       makeDup(11),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(11),
			valid:         true,
		},
		DUP12: {
			execute:       makeDup(12),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(12),
			valid:         true,
		},
		DUP13: {
			execute:       makeDup(13),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(13),
			valid:         true,
		},
		DUP14: {
			execute:       makeDup(14),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(14),
			valid:         true,
		},
		DUP15: {
			execute:       makeDup(15),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(15),
			valid:         true,
		},
		DUP16: {
			execute:       makeDup(16),
			constGas:      GasFastestStep,
			validateStack: makeDupStackFunc(16),
			valid:         true,
		},
		SWAP1: {
			execute:       makeSwap(1),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(2),
			valid:         true,
		},
		SWAP2: {
			execute:       makeSwap(2),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(3),
			valid:         true,
		},
		SWAP3: {
			execute:       makeSwap(3),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(4),
			valid:         true,
		},
		SWAP4: {
			execute:       makeSwap(4),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(5),
			valid:         true,
		},
		SWAP5: {
			execute:       makeSwap(5),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(6),
			valid:         true,
		},
		SWAP6: {
			execute:       makeSwap(6),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(7),
			valid:         true,
		},
		SWAP7: {
			execute:       makeSwap(7),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(8),
			valid:         true,
		},
		SWAP8: {
			execute:       makeSwap(8),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(9),
			valid:         true,
		},
		SWAP9: {
			execute:       makeSwap(9),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(10),
			valid:         true,
		},
		SWAP10: {
			execute:       makeSwap(10),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(11),
			valid:         true,
		},
		SWAP11: {
			execute:       makeSwap(11),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(12),
			valid:         true,
		},
		SWAP12: {
			execute:       makeSwap(12),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(13),
			valid:         true,
		},
		SWAP13: {
			execute:       makeSwap(13),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(14),
			valid:         true,
		},
		SWAP14: {
			execute:       makeSwap(14),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(15),
			valid:         true,
		},
		SWAP15: {
			execute:       makeSwap(15),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(16),
			valid:         true,
		},
		SWAP16: {
			execute:       makeSwap(16),
			constGas:      GasFastestStep,
			validateStack: makeSwapStackFunc(17),
			valid:         true,
		},
		LOG0: {
			execute:       makeLog(0),
			dynamicGas:    makeGasLog(0),
			validateStack: makeStackFunc(2, 0),
			memorySize:    memoryLog,
			valid:         true,
			writes:        true,
		},
		LOG1: {
			execute:       makeLog(1),
			dynamicGas:    makeGasLog(1),
			validateStack: makeStackFunc(3, 0),
			memorySize:    memoryLog,
			valid:         true,
			writes:        true,
		},
		LOG2: {
			execute:       makeLog(2),
			dynamicGas:    makeGasLog(2),
			validateStack: makeStackFunc(4, 0),
			memorySize:    memoryLog,
			valid:         true,
			writes:        true,
		},
		LOG3: {
			execute:       makeLog(3),
			dynamicGas:    makeGasLog(3),
			validateStack: makeStackFunc(5, 0),
			memorySize:    memoryLog,
			valid:         true,
			writes:        true,
		},
		LOG4: {
			execute:       makeLog(4),
			dynamicGas:    makeGasLog(4),
			validateStack: makeStackFunc(6, 0),
			memorySize:    memoryLog,
			valid:         true,
			writes:        true,
		},
		CREATE: {
			execute:       opCreate,
			dynamicGas:    gasCreate,
			validateStack: makeStackFunc(3, 1),
			memorySize:    memoryCreate,
			valid:         true,
			writes:        true,
			returns:       true,
		},
		CALL: {
			execute:       opCall,
			dynamicGas:    gasCall,
			validateStack: makeStackFunc(7, 1),
			memorySize:    memoryCall,
			valid:         true,
			returns:       true,
		},
		CALLCODE: {
			execute:       opCallCode,
			dynamicGas:    gasCallCode,
			validateStack: makeStackFunc(7, 1),
			memorySize:    memoryCall,
			valid:         true,
			returns:       true,
		},
		RETURN: {
			execute:       opReturn,
			dynamicGas:    gasReturn,
			validateStack: makeStackFunc(2, 0),
			memorySize:    memoryReturn,
			halts:         true,
			valid:         true,
		},
		SELFDESTRUCT: {
			execute:       opSuicide,
			dynamicGas:    gasSuicide,
			validateStack: makeStackFunc(1, 0),
			halts:         true,
			valid:         true,
			writes:        true,
		},
	}
}
