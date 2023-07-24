// Copyright JAMF Software, LLC

package client

import (
	pb "github.com/jamf/regatta-go/proto"
)

type opType int

const (
	// A default Op has opType 0, which is invalid.
	tRange opType = iota + 1
	tPut
	tDeleteRange
	tTxn
)

var noPrefixEnd = []byte{0}

// Op represents an Operation that kv can execute.
type Op struct {
	t   opType
	key []byte
	end []byte

	// for range
	limit        int64
	serializable bool
	keysOnly     bool
	countOnly    bool
	minModRev    int64
	maxModRev    int64
	minCreateRev int64
	maxCreateRev int64

	// for range, watch
	rev int64

	// for watch, put, delete
	prevKV bool

	// for put
	val []byte

	// txn
	cmps    []Cmp
	thenOps []Op
	elseOps []Op

	isOptsWithFromKey bool
	isOptsWithPrefix  bool
}

// accessors / mutators

// IsTxn returns true if the "Op" type is transaction.
func (op *Op) IsTxn() bool {
	return op.t == tTxn
}

// Txn returns the comparison(if) operations, "then" operations, and "else" operations.
func (op *Op) Txn() ([]Cmp, []Op, []Op) {
	return op.cmps, op.thenOps, op.elseOps
}

// KeyBytes returns the byte slice holding the Op's key.
func (op *Op) KeyBytes() []byte { return op.key }

// WithKeyBytes sets the byte slice for the Op's key.
func (op *Op) WithKeyBytes(key []byte) { op.key = key }

// RangeBytes returns the byte slice holding with the Op's range end, if any.
func (op *Op) RangeBytes() []byte { return op.end }

// Rev returns the requested revision, if any.
func (op *Op) Rev() int64 { return op.rev }

// IsPut returns true iff the operation is a Put.
func (op *Op) IsPut() bool { return op.t == tPut }

// IsGet returns true iff the operation is a Get.
func (op *Op) IsGet() bool { return op.t == tRange }

// IsDelete returns true iff the operation is a Delete.
func (op *Op) IsDelete() bool { return op.t == tDeleteRange }

// IsSerializable returns true if the serializable field is true.
func (op *Op) IsSerializable() bool { return op.serializable }

// IsKeysOnly returns whether keysOnly is set.
func (op *Op) IsKeysOnly() bool { return op.keysOnly }

// IsCountOnly returns whether countOnly is set.
func (op *Op) IsCountOnly() bool { return op.countOnly }

// WithRangeBytes sets the byte slice for the Op's range end.
func (op *Op) WithRangeBytes(end []byte) { op.end = end }

// ValueBytes returns the byte slice holding the Op's value, if any.
func (op *Op) ValueBytes() []byte { return op.val }

// WithValueBytes sets the byte slice for the Op's value.
func (op *Op) WithValueBytes(v []byte) { op.val = v }

func (op *Op) toRangeRequest(table string) *pb.RangeRequest {
	if op.t != tRange {
		panic("op.t != tRange")
	}
	r := &pb.RangeRequest{
		Table:             []byte(table),
		Key:               op.key,
		RangeEnd:          op.end,
		Limit:             op.limit,
		KeysOnly:          op.keysOnly,
		CountOnly:         op.countOnly,
		MinModRevision:    op.minModRev,
		MaxModRevision:    op.maxModRev,
		MinCreateRevision: op.minCreateRev,
		MaxCreateRevision: op.maxCreateRev,
		Linearizable:      !op.serializable,
	}
	return r
}

func (op *Op) toTxnRequest(table string) *pb.TxnRequest {
	thenOps := make([]*pb.RequestOp, len(op.thenOps))
	for i, tOp := range op.thenOps {
		thenOps[i] = tOp.toRequestOp()
	}
	elseOps := make([]*pb.RequestOp, len(op.elseOps))
	for i, eOp := range op.elseOps {
		elseOps[i] = eOp.toRequestOp()
	}
	cmps := make([]*pb.Compare, len(op.cmps))
	for i := range op.cmps {
		cmps[i] = (*pb.Compare)(&op.cmps[i])
	}
	return &pb.TxnRequest{Table: []byte(table), Compare: cmps, Success: thenOps, Failure: elseOps}
}

func (op *Op) toRequestOp() *pb.RequestOp {
	switch op.t {
	case tRange:
		r := &pb.RequestOp_Range{
			Key:       op.key,
			RangeEnd:  op.end,
			Limit:     op.limit,
			KeysOnly:  op.keysOnly,
			CountOnly: op.countOnly,
		}
		return &pb.RequestOp{Request: &pb.RequestOp_RequestRange{RequestRange: r}}
	case tPut:
		r := &pb.RequestOp_Put{Key: op.key, Value: op.val, PrevKv: op.prevKV}
		return &pb.RequestOp{Request: &pb.RequestOp_RequestPut{RequestPut: r}}
	case tDeleteRange:
		r := &pb.RequestOp_DeleteRange{Key: op.key, RangeEnd: op.end, PrevKv: op.prevKV}
		return &pb.RequestOp{Request: &pb.RequestOp_RequestDeleteRange{RequestDeleteRange: r}}
	default:
		panic("Unknown Op")
	}
}

func (op *Op) isWrite() bool {
	if op.t == tTxn {
		for _, tOp := range op.thenOps {
			if tOp.isWrite() {
				return true
			}
		}
		for _, tOp := range op.elseOps {
			if tOp.isWrite() {
				return true
			}
		}
		return false
	}
	return op.t != tRange
}

func NewOp() *Op {
	return &Op{key: []byte("")}
}

// OpGet returns "get" operation based on given key and operation options.
func OpGet(key string, opts ...OpOption) Op {
	// WithPrefix and WithFromKey are not supported together
	if IsOptsWithPrefix(opts) && IsOptsWithFromKey(opts) {
		panic("`WithPrefix` and `WithFromKey` cannot be set at the same time, choose one")
	}
	ret := Op{t: tRange, key: []byte(key)}
	ret.applyOpts(opts)
	return ret
}

// OpDelete returns "delete" operation based on given key and operation options.
func OpDelete(key string, opts ...OpOption) Op {
	// WithPrefix and WithFromKey are not supported together
	if IsOptsWithPrefix(opts) && IsOptsWithFromKey(opts) {
		panic("`WithPrefix` and `WithFromKey` cannot be set at the same time, choose one")
	}
	ret := Op{t: tDeleteRange, key: []byte(key)}
	ret.applyOpts(opts)
	switch {
	case ret.limit != 0:
		panic("unexpected limit in delete")
	case ret.rev != 0:
		panic("unexpected revision in delete")
	case ret.serializable:
		panic("unexpected serializable in delete")
	case ret.countOnly:
		panic("unexpected countOnly in delete")
	case ret.minModRev != 0, ret.maxModRev != 0:
		panic("unexpected mod revision filter in delete")
	case ret.minCreateRev != 0, ret.maxCreateRev != 0:
		panic("unexpected create revision filter in delete")
	}
	return ret
}

// OpPut returns "put" operation based on given key-value and operation options.
func OpPut(key, val string, opts ...OpOption) Op {
	ret := Op{t: tPut, key: []byte(key), val: []byte(val)}
	ret.applyOpts(opts)
	switch {
	case ret.end != nil:
		panic("unexpected range in put")
	case ret.limit != 0:
		panic("unexpected limit in put")
	case ret.rev != 0:
		panic("unexpected revision in put")
	case ret.serializable:
		panic("unexpected serializable in put")
	case ret.countOnly:
		panic("unexpected countOnly in put")
	case ret.minModRev != 0, ret.maxModRev != 0:
		panic("unexpected mod revision filter in put")
	case ret.minCreateRev != 0, ret.maxCreateRev != 0:
		panic("unexpected create revision filter in put")
	}
	return ret
}

// OpTxn returns "txn" operation based on given transaction conditions.
func OpTxn(cmps []Cmp, thenOps []Op, elseOps []Op) Op {
	return Op{t: tTxn, cmps: cmps, thenOps: thenOps, elseOps: elseOps}
}

func (op *Op) applyOpts(opts []OpOption) {
	for _, opt := range opts {
		opt(op)
	}
}

// OpOption configures Operations like Get, Put, Delete.
type OpOption func(*Op)

// WithLimit limits the number of results to return from 'Get' request.
// If WithLimit is given a 0 limit, it is treated as no limit.
func WithLimit(n int64) OpOption { return func(op *Op) { op.limit = n } }

// WithRev specifies the store revision for 'Get' request.
// Or the start revision of 'Watch' request.
func WithRev(rev int64) OpOption { return func(op *Op) { op.rev = rev } }

// GetPrefixRangeEnd gets the range end of the prefix.
// 'Get(foo, WithPrefix())' is equal to 'Get(foo, WithRange(GetPrefixRangeEnd(foo))'.
func GetPrefixRangeEnd(prefix string) string {
	return string(getPrefix([]byte(prefix)))
}

func getPrefix(key []byte) []byte {
	end := make([]byte, len(key))
	copy(end, key)
	for i := len(end) - 1; i >= 0; i-- {
		if end[i] < 0xff {
			end[i] = end[i] + 1
			end = end[:i+1]
			return end
		}
	}
	// next prefix does not exist (e.g., 0xffff);
	// default to WithFromKey policy
	return noPrefixEnd
}

// WithPrefix enables 'Get', 'Delete', or 'Watch' requests to operate
// on the keys with matching prefix. For example, 'Get(foo, WithPrefix())'
// can return 'foo1', 'foo2', and so on.
func WithPrefix() OpOption {
	return func(op *Op) {
		op.isOptsWithPrefix = true
		if len(op.key) == 0 {
			op.key, op.end = []byte{0}, []byte{0}
			return
		}
		op.end = getPrefix(op.key)
	}
}

// WithRange specifies the range of 'Get', 'Delete', 'Watch' requests.
// For example, 'Get' requests with 'WithRange(end)' returns
// the keys in the range [key, end).
// endKey must be lexicographically greater than start key.
func WithRange(endKey string) OpOption {
	return func(op *Op) { op.end = []byte(endKey) }
}

// WithFromKey specifies the range of 'Get', 'Delete', 'Watch' requests
// to be equal or greater than the key in the argument.
func WithFromKey() OpOption {
	return func(op *Op) {
		if len(op.key) == 0 {
			op.key = []byte{0}
		}
		op.end = []byte("\x00")
		op.isOptsWithFromKey = true
	}
}

// WithSerializable makes 'Get' request serializable. By default,
// it's linearizable. Serializable requests are better for lower latency
// requirement.
func WithSerializable() OpOption {
	return func(op *Op) { op.serializable = true }
}

// WithKeysOnly makes the 'Get' request return only the keys and the corresponding
// values will be omitted.
func WithKeysOnly() OpOption {
	return func(op *Op) { op.keysOnly = true }
}

// WithCountOnly makes the 'Get' request return only the count of keys.
func WithCountOnly() OpOption {
	return func(op *Op) { op.countOnly = true }
}

// WithPrevKV gets the previous key-value pair before the event happens. If the previous KV is already compacted,
// nothing will be returned.
func WithPrevKV() OpOption {
	return func(op *Op) {
		op.prevKV = true
	}
}

// IsOptsWithPrefix returns true if WithPrefix option is called in the given opts.
func IsOptsWithPrefix(opts []OpOption) bool {
	ret := NewOp()
	for _, opt := range opts {
		opt(ret)
	}

	return ret.isOptsWithPrefix
}

// IsOptsWithFromKey returns true if WithFromKey option is called in the given opts.
func IsOptsWithFromKey(opts []OpOption) bool {
	ret := NewOp()
	for _, opt := range opts {
		opt(ret)
	}

	return ret.isOptsWithFromKey
}

type CompareTarget int

type CompareResult int

const (
	CompareVersion CompareTarget = iota
	CompareCreated
	CompareModified
	CompareValue
)

type Cmp pb.Compare

func Compare(cmp Cmp, result string, v interface{}) Cmp {
	var r pb.Compare_CompareResult

	switch result {
	case "=":
		r = pb.Compare_EQUAL
	case "!=":
		r = pb.Compare_NOT_EQUAL
	case ">":
		r = pb.Compare_GREATER
	case "<":
		r = pb.Compare_LESS
	default:
		panic("Unknown result op")
	}

	cmp.Result = r
	switch cmp.Target {
	case pb.Compare_VALUE:
		val, ok := v.(string)
		if !ok {
			panic("bad compare value")
		}
		cmp.TargetUnion = &pb.Compare_Value{Value: []byte(val)}
	default:
		panic("Unknown compare type")
	}
	return cmp
}

func Value(key string) Cmp {
	return Cmp{Key: []byte(key), Target: pb.Compare_VALUE}
}

// KeyBytes returns the byte slice holding with the comparison key.
func (cmp *Cmp) KeyBytes() []byte { return cmp.Key }

// WithKeyBytes sets the byte slice for the comparison key.
func (cmp *Cmp) WithKeyBytes(key []byte) { cmp.Key = key }

// ValueBytes returns the byte slice holding the comparison value, if any.
func (cmp *Cmp) ValueBytes() []byte {
	if tu, ok := cmp.TargetUnion.(*pb.Compare_Value); ok {
		return tu.Value
	}
	return nil
}

// WithValueBytes sets the byte slice for the comparison's value.
func (cmp *Cmp) WithValueBytes(v []byte) { cmp.TargetUnion.(*pb.Compare_Value).Value = v }

// WithRange sets the comparison to scan the range [key, end).
func (cmp Cmp) WithRange(end string) Cmp {
	cmp.RangeEnd = []byte(end)
	return cmp
}

// WithPrefix sets the comparison to scan all keys prefixed by the key.
func (cmp Cmp) WithPrefix() Cmp {
	cmp.RangeEnd = getPrefix(cmp.Key)
	return cmp
}
