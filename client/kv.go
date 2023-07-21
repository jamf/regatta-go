// Copyright JAMF Software, LLC

package client

import (
	"context"
	"math/rand"
	"time"

	pb "github.com/jamf/regatta-go/proto"
	"google.golang.org/grpc"
)

type (
	PutResponse    pb.PutResponse
	GetResponse    pb.RangeResponse
	DeleteResponse pb.DeleteRangeResponse
	TxnResponse    pb.TxnResponse
)

type Table interface {
	// Put puts a key-value pair into regatta.
	// Note that key,value can be plain bytes array and string is
	// an immutable representation of that bytes array.
	// To get a string of bytes, do string([]byte{0x10, 0x20}).
	Put(ctx context.Context, key, val string, opts ...OpOption) (*PutResponse, error)

	// Get retrieves keys.
	// By default, Get will return the value for "key", if any.
	// When passed WithRange(end), Get will return the keys in the range [key, end).
	// When passed WithFromKey(), Get returns keys greater than or equal to key.
	// When passed WithRev(rev) with rev > 0, Get retrieves keys at the given revision;
	// if the required revision is compacted, the request will fail with ErrCompacted .
	// When passed WithLimit(limit), the number of returned keys is bounded by limit.
	// When passed WithSort(), the keys will be sorted.
	Get(ctx context.Context, key string, opts ...OpOption) (*GetResponse, error)

	// Delete deletes a key, or optionally using WithRange(end), [key, end).
	Delete(ctx context.Context, key string, opts ...OpOption) (*DeleteResponse, error)

	// Txn creates a transaction.
	Txn(ctx context.Context) Txn
}

type KV interface {
	// Put puts a key-value pair into regatta.
	// Note that key,value can be plain bytes array and string is
	// an immutable representation of that bytes array.
	// To get a string of bytes, do string([]byte{0x10, 0x20}).
	Put(ctx context.Context, table, key, val string, opts ...OpOption) (*PutResponse, error)

	// Get retrieves keys.
	// By default, Get will return the value for "key", if any.
	// When passed WithRange(end), Get will return the keys in the range [key, end).
	// When passed WithFromKey(), Get returns keys greater than or equal to key.
	// When passed WithRev(rev) with rev > 0, Get retrieves keys at the given revision;
	// if the required revision is compacted, the request will fail with ErrCompacted .
	// When passed WithLimit(limit), the number of returned keys is bounded by limit.
	// When passed WithSort(), the keys will be sorted.
	Get(ctx context.Context, table, key string, opts ...OpOption) (*GetResponse, error)

	// Delete deletes a key, or optionally using WithRange(end), [key, end).
	Delete(ctx context.Context, table, key string, opts ...OpOption) (*DeleteResponse, error)

	// Do apply a single Op on KV without a transaction.
	// Do is useful when creating arbitrary operations to be issued at a
	// later time; the user can range over the operations, calling Do to
	// execute them. Get/Put/Delete, on the other hand, are best suited
	// for when the operation should be issued at the time of declaration.
	Do(ctx context.Context, op Op) (OpResponse, error)

	// Table is a convenience function that provides requests scoped to a single table.
	Table(name string) Table

	// Txn creates a transaction.
	Txn(ctx context.Context, table string) Txn
}

type OpResponse struct {
	put *PutResponse
	get *GetResponse
	del *DeleteResponse
	txn *TxnResponse
}

func (op OpResponse) Put() *PutResponse    { return op.put }
func (op OpResponse) Get() *GetResponse    { return op.get }
func (op OpResponse) Del() *DeleteResponse { return op.del }
func (op OpResponse) Txn() *TxnResponse    { return op.txn }

func (resp *PutResponse) OpResponse() OpResponse {
	return OpResponse{put: resp}
}

func (resp *GetResponse) OpResponse() OpResponse {
	return OpResponse{get: resp}
}

func (resp *DeleteResponse) OpResponse() OpResponse {
	return OpResponse{del: resp}
}

func (resp *TxnResponse) OpResponse() OpResponse {
	return OpResponse{txn: resp}
}

type kv struct {
	remote   pb.KVClient
	callOpts []grpc.CallOption
}

func NewKV(c *Client) KV {
	return &kv{remote: &retryKVClient{client: pb.NewKVClient(c.conn)}, callOpts: c.callOpts}
}

func (kv *kv) Put(ctx context.Context, table, key, val string, opts ...OpOption) (*PutResponse, error) {
	r, err := kv.Do(ctx, opPut(table, key, val, opts...))
	return r.put, toErr(ctx, err)
}

func (kv *kv) Get(ctx context.Context, table, key string, opts ...OpOption) (*GetResponse, error) {
	r, err := kv.Do(ctx, opGet(table, key, opts...))
	return r.get, toErr(ctx, err)
}

func (kv *kv) Delete(ctx context.Context, table, key string, opts ...OpOption) (*DeleteResponse, error) {
	r, err := kv.Do(ctx, opDelete(table, key, opts...))
	return r.del, toErr(ctx, err)
}

func (kv *kv) Txn(ctx context.Context, table string) Txn {
	return &txn{
		kv:       kv,
		ctx:      ctx,
		table:    table,
		callOpts: kv.callOpts,
	}
}

func (kv *kv) Table(name string) Table {
	return &table{
		kv:       kv,
		table:    name,
		callOpts: kv.callOpts,
	}
}

func (kv *kv) Do(ctx context.Context, op Op) (OpResponse, error) {
	var err error
	switch op.t {
	case tRange:
		var resp *pb.RangeResponse
		resp, err = kv.remote.Range(ctx, op.toRangeRequest(), kv.callOpts...)
		if err == nil {
			return OpResponse{get: (*GetResponse)(resp)}, nil
		}
	case tPut:
		var resp *pb.PutResponse
		r := &pb.PutRequest{Table: op.table, Key: op.key, Value: op.val, PrevKv: op.prevKV}
		resp, err = kv.remote.Put(ctx, r, kv.callOpts...)
		if err == nil {
			return OpResponse{put: (*PutResponse)(resp)}, nil
		}
	case tDeleteRange:
		var resp *pb.DeleteRangeResponse
		r := &pb.DeleteRangeRequest{Table: op.table, Key: op.key, RangeEnd: op.end, PrevKv: op.prevKV}
		resp, err = kv.remote.DeleteRange(ctx, r, kv.callOpts...)
		if err == nil {
			return OpResponse{del: (*DeleteResponse)(resp)}, nil
		}
	case tTxn:
		var resp *pb.TxnResponse
		resp, err = kv.remote.Txn(ctx, op.toTxnRequest(), kv.callOpts...)
		if err == nil {
			return OpResponse{txn: (*TxnResponse)(resp)}, nil
		}
	default:
		panic("Unknown op")
	}
	return OpResponse{}, toErr(ctx, err)
}

type table struct {
	kv       *kv
	table    string
	callOpts []grpc.CallOption
}

func (t *table) Put(ctx context.Context, key, val string, opts ...OpOption) (*PutResponse, error) {
	r, err := t.kv.Do(ctx, opPut(t.table, key, val, opts...))
	return r.put, toErr(ctx, err)
}

func (t *table) Get(ctx context.Context, key string, opts ...OpOption) (*GetResponse, error) {
	r, err := t.kv.Do(ctx, opGet(t.table, key, opts...))
	return r.get, toErr(ctx, err)
}

func (t *table) Delete(ctx context.Context, key string, opts ...OpOption) (*DeleteResponse, error) {
	r, err := t.kv.Do(ctx, opDelete(t.table, key, opts...))
	return r.del, toErr(ctx, err)
}

func (t *table) Txn(ctx context.Context) Txn {
	return &txn{
		kv:       t.kv,
		ctx:      ctx,
		table:    t.table,
		callOpts: t.kv.callOpts,
	}
}

type retryKVClient struct {
	client pb.KVClient
}

func (rkv *retryKVClient) Range(ctx context.Context, in *pb.RangeRequest, opts ...grpc.CallOption) (resp *pb.RangeResponse, err error) {
	return rkv.client.Range(ctx, in, append(opts, withRetryPolicy(repeatable))...)
}

func (rkv *retryKVClient) Put(ctx context.Context, in *pb.PutRequest, opts ...grpc.CallOption) (resp *pb.PutResponse, err error) {
	return rkv.client.Put(ctx, in, opts...)
}

func (rkv *retryKVClient) DeleteRange(ctx context.Context, in *pb.DeleteRangeRequest, opts ...grpc.CallOption) (resp *pb.DeleteRangeResponse, err error) {
	return rkv.client.DeleteRange(ctx, in, opts...)
}

func (rkv *retryKVClient) Txn(ctx context.Context, in *pb.TxnRequest, opts ...grpc.CallOption) (resp *pb.TxnResponse, err error) {
	return rkv.client.Txn(ctx, in, opts...)
}

type retryPolicy uint8

const (
	repeatable retryPolicy = iota
	nonRepeatable
)

func (rp retryPolicy) String() string {
	switch rp {
	case repeatable:
		return "repeatable"
	case nonRepeatable:
		return "nonRepeatable"
	default:
		return "UNKNOWN"
	}
}

type backoffFunc func(attempt uint) time.Duration

type options struct {
	retryPolicy retryPolicy
	max         uint
	backoffFunc backoffFunc
	retryAuth   bool
}

var defaultOptions = &options{
	retryPolicy: nonRepeatable,
	max:         0, // disable
	backoffFunc: backoffLinearWithJitter(50*time.Millisecond /*jitter*/, 0.10),
	retryAuth:   true,
}

type retryOption struct {
	grpc.EmptyCallOption
	applyFunc func(opt *options)
}

// withRetryPolicy sets the retry policy of this call.
func withRetryPolicy(rp retryPolicy) retryOption {
	return retryOption{applyFunc: func(o *options) {
		o.retryPolicy = rp
	}}
}

// withMax sets the maximum number of retries on this call, or this interceptor.
func withMax(maxRetries uint) retryOption {
	return retryOption{applyFunc: func(o *options) {
		o.max = maxRetries
	}}
}

// WithBackoff sets the `BackoffFunc` used to control time between retries.
func withBackoff(bf backoffFunc) retryOption {
	return retryOption{applyFunc: func(o *options) {
		o.backoffFunc = bf
	}}
}

func backoffLinearWithJitter(waitBetween time.Duration, jitterFraction float64) backoffFunc {
	return func(attempt uint) time.Duration {
		return jitterUp(waitBetween, jitterFraction)
	}
}

func jitterUp(duration time.Duration, jitter float64) time.Duration {
	//nolint:gosec
	multiplier := jitter * (rand.Float64()*2 - 1)
	return time.Duration(float64(duration) * (1 + multiplier))
}
