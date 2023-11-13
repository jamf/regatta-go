// Copyright JAMF Software, LLC

package client

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/jamf/regatta-go/internal/proto"
	"google.golang.org/grpc"
)

type (
	KeyValue       regattapb.KeyValue
	ResponseHeader regattapb.ResponseHeader
	ResponseOp     regattapb.ResponseOp
)

type GetResponse struct {
	Header *ResponseHeader `json:"header,omitempty"`
	// kvs is the list of key-value pairs matched by the range request.
	// kvs is empty when count is requested.
	Kvs []*KeyValue `json:"kvs,omitempty"`
	// more indicates if there are more keys to return in the requested range.
	More bool `json:"more,omitempty"`
	// count is set to the number of keys within the range when requested.
	Count int64 `json:"count,omitempty"`
}

func (resp *GetResponse) String() string {
	enc, _ := json.Marshal(resp)
	return string(enc)
}

type DeleteResponse struct {
	Header *ResponseHeader `json:"header,omitempty"`
	// deleted is the number of keys deleted by the delete range request.
	Deleted int64 `json:"deleted,omitempty"`
	// if prev_kv is set in the request, the previous key-value pairs will be returned.
	PrevKvs []*KeyValue `json:"prev_kvs,omitempty"`
}

func (resp *DeleteResponse) String() string {
	enc, _ := json.Marshal(resp)
	return string(enc)
}

type PutResponse struct {
	Header *ResponseHeader `json:"header,omitempty"`
	// if prev_kv is set in the request, the previous key-value pair will be returned.
	PrevKv *KeyValue `json:"prev_kv,omitempty"`
}

func (resp *PutResponse) String() string {
	enc, _ := json.Marshal(resp)
	return string(enc)
}

type TxnResponse struct {
	Header *ResponseHeader `json:"header,omitempty"`
	// succeeded is set to true if the compare evaluated to true or false otherwise.
	Succeeded bool `json:"succeeded,omitempty"`
	// responses is a list of responses corresponding to the results from applying
	// success if succeeded is true or failure if succeeded is false.
	Responses []*ResponseOp `json:"responses,omitempty"`
}

func (resp *TxnResponse) String() string {
	enc, _ := json.Marshal(resp)
	return string(enc)
}

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
	Do(ctx context.Context, table string, op Op) (OpResponse, error)

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

type hookedKV struct {
	KV
	hook HookKVOpDo
}

func (kv *hookedKV) Do(ctx context.Context, table string, op Op) (OpResponse, error) {
	return kv.hook.OnKVCall(ctx, table, op, kv.KV.Do)
}

func (kv *hookedKV) Txn(ctx context.Context, table string) Txn {
	return &txn{
		kv:    kv,
		ctx:   ctx,
		table: table,
	}
}

func (kv *hookedKV) Table(name string) Table {
	return &table{
		kv:    kv,
		table: name,
	}
}

type kv struct {
	remote   regattapb.KVClient
	callOpts []grpc.CallOption
}

func newKV(c *Client) KV {
	var k KV = &kv{remote: &retryKVClient{client: regattapb.NewKVClient(c.conn)}, callOpts: c.callOpts}
	for _, h := range getHooks[HookKVOpDo](c.cfg.Hooks) {
		k = &hookedKV{KV: k, hook: h}
	}
	return k
}

func (kv *kv) Put(ctx context.Context, table, key, val string, opts ...OpOption) (*PutResponse, error) {
	r, err := kv.Do(ctx, table, OpPut(key, val, opts...))
	return r.put, toErr(ctx, err)
}

func (kv *kv) Get(ctx context.Context, table, key string, opts ...OpOption) (*GetResponse, error) {
	r, err := kv.Do(ctx, table, OpGet(key, opts...))
	return r.get, toErr(ctx, err)
}

func (kv *kv) Delete(ctx context.Context, table, key string, opts ...OpOption) (*DeleteResponse, error) {
	r, err := kv.Do(ctx, table, OpDelete(key, opts...))
	return r.del, toErr(ctx, err)
}

func (kv *kv) Txn(ctx context.Context, table string) Txn {
	return &txn{
		kv:    kv,
		ctx:   ctx,
		table: table,
	}
}

func (kv *kv) Table(name string) Table {
	return &table{
		kv:       kv,
		table:    name,
		callOpts: kv.callOpts,
	}
}

func (kv *kv) Do(ctx context.Context, table string, op Op) (OpResponse, error) {
	var err error
	switch op.t {
	case tRange:
		var resp *regattapb.RangeResponse
		resp, err = kv.remote.Range(ctx, op.toRangeRequest(table), kv.callOpts...)
		if err == nil {
			return OpResponse{get: &GetResponse{
				Header: (*ResponseHeader)(resp.Header),
				Kvs:    convKeyValues(resp.Kvs),
				More:   resp.More,
				Count:  resp.Count,
			}}, nil
		}
	case tPut:
		var resp *regattapb.PutResponse
		r := &regattapb.PutRequest{Table: []byte(table), Key: op.key, Value: op.val, PrevKv: op.prevKV}
		resp, err = kv.remote.Put(ctx, r, kv.callOpts...)
		if err == nil {
			return OpResponse{put: &PutResponse{
				Header: (*ResponseHeader)(resp.Header),
				PrevKv: (*KeyValue)(resp.PrevKv),
			}}, nil
		}
	case tDeleteRange:
		var resp *regattapb.DeleteRangeResponse
		r := &regattapb.DeleteRangeRequest{Table: []byte(table), Key: op.key, RangeEnd: op.end, PrevKv: op.prevKV}
		resp, err = kv.remote.DeleteRange(ctx, r, kv.callOpts...)
		if err == nil {
			return OpResponse{del: &DeleteResponse{
				Header:  (*ResponseHeader)(resp.Header),
				Deleted: resp.Deleted,
				PrevKvs: convKeyValues(resp.PrevKvs),
			}}, nil
		}
	case tTxn:
		var resp *regattapb.TxnResponse
		resp, err = kv.remote.Txn(ctx, op.toTxnRequest(table), kv.callOpts...)
		if err == nil {
			return OpResponse{txn: &TxnResponse{
				Header:    (*ResponseHeader)(resp.Header),
				Succeeded: resp.Succeeded,
				Responses: convResponseOps(resp.Responses),
			}}, nil
		}
	default:
		panic("Unknown op")
	}
	return OpResponse{}, toErr(ctx, err)
}

func convResponseOps(in []*regattapb.ResponseOp) (out []*ResponseOp) {
	for _, t := range in {
		out = append(out, (*ResponseOp)(t))
	}
	return
}

func convKeyValues(in []*regattapb.KeyValue) (out []*KeyValue) {
	for _, t := range in {
		out = append(out, (*KeyValue)(t))
	}
	return
}

type table struct {
	kv       KV
	table    string
	callOpts []grpc.CallOption
}

func (t *table) Put(ctx context.Context, key, val string, opts ...OpOption) (*PutResponse, error) {
	r, err := t.kv.Do(ctx, t.table, OpPut(key, val, opts...))
	return r.put, toErr(ctx, err)
}

func (t *table) Get(ctx context.Context, key string, opts ...OpOption) (*GetResponse, error) {
	r, err := t.kv.Do(ctx, t.table, OpGet(key, opts...))
	return r.get, toErr(ctx, err)
}

func (t *table) Delete(ctx context.Context, key string, opts ...OpOption) (*DeleteResponse, error) {
	r, err := t.kv.Do(ctx, t.table, OpDelete(key, opts...))
	return r.del, toErr(ctx, err)
}

func (t *table) Txn(ctx context.Context) Txn {
	return &txn{
		kv:    t.kv,
		ctx:   ctx,
		table: t.table,
	}
}

type retryKVClient struct {
	client regattapb.KVClient
}

func (rkv *retryKVClient) Range(ctx context.Context, in *regattapb.RangeRequest, opts ...grpc.CallOption) (resp *regattapb.RangeResponse, err error) {
	return rkv.client.Range(ctx, in, append(opts, withRetryPolicy(repeatable))...)
}

func (rkv *retryKVClient) Put(ctx context.Context, in *regattapb.PutRequest, opts ...grpc.CallOption) (resp *regattapb.PutResponse, err error) {
	return rkv.client.Put(ctx, in, opts...)
}

func (rkv *retryKVClient) DeleteRange(ctx context.Context, in *regattapb.DeleteRangeRequest, opts ...grpc.CallOption) (resp *regattapb.DeleteRangeResponse, err error) {
	return rkv.client.DeleteRange(ctx, in, opts...)
}

func (rkv *retryKVClient) Txn(ctx context.Context, in *regattapb.TxnRequest, opts ...grpc.CallOption) (resp *regattapb.TxnResponse, err error) {
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
