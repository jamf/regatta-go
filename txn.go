// Copyright JAMF Software, LLC

package client

import (
	"context"
	"sync"
)

type Txn interface {
	// If takes a list of comparison. If all comparisons passed in succeed,
	// the operations passed into Then() will be executed. Or the operations
	// passed into Else() will be executed.
	If(cs ...Cmp) Txn

	// Then takes a list of operations. The Ops list will be executed, if the
	// comparisons passed in If() succeed.
	Then(ops ...Op) Txn

	// Else takes a list of operations. The Ops list will be executed, if the
	// comparisons passed in If() fail.
	Else(ops ...Op) Txn

	// Commit tries to commit the transaction.
	Commit() (*TxnResponse, error)
}

type txn struct {
	kv    KV
	ctx   context.Context
	table string

	mu    sync.Mutex
	cif   bool
	cthen bool

	celse bool

	isWrite bool

	cmps []Cmp
	sus  []Op
	fas  []Op
}

func (txn *txn) If(cs ...Cmp) Txn {
	txn.mu.Lock()
	defer txn.mu.Unlock()

	if txn.cif {
		panic("cannot call If twice!")
	}

	if txn.cthen {
		panic("cannot call If after Then!")
	}

	if txn.celse {
		panic("cannot call If after Else!")
	}

	txn.cif = true

	txn.cmps = append(txn.cmps, cs...)

	return txn
}

func (txn *txn) Then(ops ...Op) Txn {
	txn.mu.Lock()
	defer txn.mu.Unlock()

	if txn.cthen {
		panic("cannot call Then twice!")
	}
	if txn.celse {
		panic("cannot call Then after Else!")
	}

	txn.cthen = true

	for _, op := range ops {
		txn.isWrite = txn.isWrite || op.isWrite()
		txn.sus = append(txn.sus, op)
	}

	return txn
}

func (txn *txn) Else(ops ...Op) Txn {
	txn.mu.Lock()
	defer txn.mu.Unlock()

	if txn.celse {
		panic("cannot call Else twice!")
	}

	txn.celse = true

	for _, op := range ops {
		txn.isWrite = txn.isWrite || op.isWrite()
		txn.fas = append(txn.fas, op)
	}

	return txn
}

func (txn *txn) Commit() (*TxnResponse, error) {
	txn.mu.Lock()
	defer txn.mu.Unlock()

	do, err := txn.kv.Do(txn.ctx, txn.table, OpTxn(txn.cmps, txn.sus, txn.fas))
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, toErr(txn.ctx, err)
	}

	return &TxnResponse{
		Header:    do.txn.Header,
		Succeeded: do.txn.Succeeded,
		Responses: do.txn.Responses,
	}, nil
}
