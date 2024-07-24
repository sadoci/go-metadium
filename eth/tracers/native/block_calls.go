// Copyright 2021 The go-ethereum Authors
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

package native

import (
	"bytes"
	"encoding/json"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
)

func init() {
	tracers.DefaultDirectory.Register("blockCallsTracer", newBlockCallsTracer, false)
}

type blockCallsTracer struct {
	callstacks [][]callFrame
	callstack  []callFrame
	config     blockCallsTracerConfig
	gasLimit   uint64
	depth      int
	interrupt  atomic.Bool // Atomic flag to signal execution interruption
	reason     error       // Textual reason for the interruption
}

type blockCallsTracerConfig struct {
	OnlyTopCall bool `json:"onlyTopCall"` // If true, call tracer won't collect any subcalls
	WithLog     bool `json:"withLog"`     // If true, call tracer will collect event logs
}

// newBlockCallTracer returns a native go tracer which tracks
// call frames of txs of a block, and implements vm.EVMLogger.
func newBlockCallsTracer(ctx *tracers.Context, cfg json.RawMessage) (*tracers.Tracer, error) {
	t, err := newBlockCallsTracerObject(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &tracers.Tracer{
		Hooks: &tracing.Hooks{
			OnTxStart:    t.OnTxStart,
			OnTxEnd:      t.OnTxEnd,
			OnEnter:      t.OnEnter,
			OnExit:       t.OnExit,
			OnLog:        t.OnLog,
			OnBlockStart: t.OnBlockStart,
		},
		GetResult: t.GetResult,
		Stop:      t.Stop,
	}, nil
}

func newBlockCallsTracerObject(ctx *tracers.Context, cfg json.RawMessage) (*blockCallsTracer, error) {
	var config blockCallsTracerConfig
	if cfg != nil {
		if err := json.Unmarshal(cfg, &config); err != nil {
			return nil, err
		}
	}
	return &blockCallsTracer{
		config: config,
	}, nil
}

func (t *blockCallsTracer) OnBlockStart(event tracing.BlockEvent) {
	t.callstacks = nil
	t.interrupt = atomic.Bool{}
	t.reason = nil
}

// OnEnter is called when EVM enters a new scope (via call, create or selfdestruct).
func (t *blockCallsTracer) OnEnter(depth int, typ byte, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	t.depth = depth
	if t.config.OnlyTopCall && depth > 0 {
		return
	}
	// Skip if tracing was interrupted
	if t.interrupt.Load() {
		return
	}

	toCopy := to
	call := callFrame{
		Type:  vm.OpCode(typ),
		From:  from,
		To:    &toCopy,
		Input: common.CopyBytes(input),
		Gas:   gas,
		Value: value,
	}
	if depth == 0 {
		call.Gas = t.gasLimit
	}
	t.callstack = append(t.callstack, call)

}

// OnExit is called when EVM exits a scope, even if the scope didn't
// execute any code.
func (t *blockCallsTracer) OnExit(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
	if depth == 0 {
		t.captureEnd(output, gasUsed, err, reverted)
		return
	}

	t.depth = depth - 1
	if t.config.OnlyTopCall {
		return
	}

	size := len(t.callstack)
	if size <= 1 {
		return
	}
	// Pop call.
	call := t.callstack[size-1]
	t.callstack = t.callstack[:size-1]
	size -= 1

	call.GasUsed = gasUsed
	call.processOutput(output, err, reverted)
	// Nest call into parent.
	t.callstack[size-1].Calls = append(t.callstack[size-1].Calls, call)
}

func (t *blockCallsTracer) captureEnd(output []byte, gasUsed uint64, err error, reverted bool) {
	if len(t.callstack) != 1 {
		return
	}
	t.callstack[0].processOutput(output, err, reverted)
}

func (t *blockCallsTracer) OnTxStart(env *tracing.VMContext, tx *types.Transaction, from common.Address) {
	t.callstack = make([]callFrame, 1)
	t.gasLimit = tx.Gas()
	t.depth = 0
	t.interrupt = atomic.Bool{}
	t.reason = nil
}

func (t *blockCallsTracer) OnTxEnd(receipt *types.Receipt, err error) {
	// Error happened during tx validation.
	if err != nil {
		return
	}
	t.callstack[0].GasUsed = receipt.GasUsed
	if t.config.WithLog {
		// Logs are not emitted when the call fails
		clearFailedLogs(&t.callstack[0], false)
	}
	t.callstacks = append(t.callstacks, t.callstack)
	t.callstack = nil
}

func (t *blockCallsTracer) OnLog(log *types.Log) {
	// Only logs need to be captured via opcode processing
	if !t.config.WithLog {
		return
	}
	// Avoid processing nested calls when only caring about top call
	if t.config.OnlyTopCall && t.depth > 0 {
		return
	}
	// Skip if tracing was interrupted
	if t.interrupt.Load() {
		return
	}
	l := callLog{
		Address:  log.Address,
		Topics:   log.Topics,
		Data:     log.Data,
		Position: hexutil.Uint(len(t.callstack[len(t.callstack)-1].Calls)),
	}
	t.callstack[len(t.callstack)-1].Logs = append(t.callstack[len(t.callstack)-1].Logs, l)
}

// GetResult returns the json-encoded nested list of call traces, and any
// error arising from the encoding or forceful termination (via `Stop`).
func (t *blockCallsTracer) GetResult() (json.RawMessage, error) {
	res, err := json.Marshal(t.callstacks)
	if err != nil {
		return nil, err
	}
	return res, t.reason
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *blockCallsTracer) Stop(err error) {
	t.reason = err
	t.interrupt.Store(true)
}

func (t *blockCallsTracer) GetBytes() ([]byte, error) {
	var bb bytes.Buffer
	bb.Write([]byte("["))
	for i, s := range t.callstacks {
		o, e := json.Marshal(s)
		if e != nil {
			return nil, e
		}
		if i != 0 {
			bb.Write([]byte(","))
		}
		bb.Write(o)
	}
	bb.Write([]byte("]"))
	return bb.Bytes(), t.reason
}
