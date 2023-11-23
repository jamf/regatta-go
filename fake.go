// Copyright JAMF Software, LLC

package client

import (
	"context"
	"net"

	regattapb "github.com/jamf/regatta-go/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// FakeResponse holds response fields returned by the fake client.
type FakeResponse struct {
	Response OpResponse
	Err      error
}

type fakeKVServer struct {
	regattapb.UnimplementedKVServer

	fakeResponses    []FakeResponse
	fakeResponsesIdx int
}

func (f *fakeKVServer) Range(context.Context, *regattapb.RangeRequest) (*regattapb.RangeResponse, error) {
	i := f.fakeResponsesIdx % len(f.fakeResponses)
	f.fakeResponsesIdx++
	resp := f.fakeResponses[i]

	if resp.Err != nil {
		return nil, resp.Err
	}

	r := resp.Response.Get()
	return &regattapb.RangeResponse{
		Header: convToHeaderProto(r.Header),
		Kvs:    convKeyValuesToProto(r.Kvs),
		More:   r.More,
		Count:  r.Count,
	}, nil
}

func (f *fakeKVServer) Put(context.Context, *regattapb.PutRequest) (*regattapb.PutResponse, error) {
	i := f.fakeResponsesIdx % len(f.fakeResponses)
	f.fakeResponsesIdx++
	resp := f.fakeResponses[i]

	if resp.Err != nil {
		return nil, resp.Err
	}

	r := resp.Response.Put()
	var kv *regattapb.KeyValue
	if r.PrevKv != nil {
		kv = &regattapb.KeyValue{
			Key:            r.PrevKv.Key,
			CreateRevision: r.PrevKv.CreateRevision,
			ModRevision:    r.PrevKv.ModRevision,
			Value:          r.PrevKv.Value,
		}
	}
	return &regattapb.PutResponse{
		Header: convToHeaderProto(r.Header),
		PrevKv: kv,
	}, nil
}

func (f *fakeKVServer) DeleteRange(context.Context, *regattapb.DeleteRangeRequest) (*regattapb.DeleteRangeResponse, error) {
	i := f.fakeResponsesIdx % len(f.fakeResponses)
	f.fakeResponsesIdx++
	resp := f.fakeResponses[i]

	if resp.Err != nil {
		return nil, resp.Err
	}

	r := resp.Response.Del()
	return &regattapb.DeleteRangeResponse{
		Header:  convToHeaderProto(r.Header),
		Deleted: r.Deleted,
		PrevKvs: convKeyValuesToProto(r.PrevKvs),
	}, nil
}

func (f *fakeKVServer) Txn(context.Context, *regattapb.TxnRequest) (*regattapb.TxnResponse, error) {
	i := f.fakeResponsesIdx % len(f.fakeResponses)
	f.fakeResponsesIdx++
	resp := f.fakeResponses[i]

	if resp.Err != nil {
		return nil, resp.Err
	}

	r := resp.Response.Txn()
	return &regattapb.TxnResponse{
		Header:    convToHeaderProto(r.Header),
		Succeeded: r.Succeeded,
		Responses: convResponsesToProto(r.Responses),
	}, nil
}

// FakeClient provides a fake regatta client. This client is intended to be used only in tests.
type FakeClient struct {
	listener *bufconn.Listener
}

// NewFake creates an instance of FakeClient, including a function for client termination.
func NewFake(fakeResponses ...FakeResponse) (*FakeClient, context.CancelFunc) {
	srv := grpc.NewServer()
	regattapb.RegisterKVServer(srv, &fakeKVServer{fakeResponses: fakeResponses})

	l := bufconn.Listen(1024)
	go srv.Serve(l)

	return &FakeClient{l}, srv.Stop
}

// Client returns a new instance of regatta client targeting the fake regatta server.
func (c *FakeClient) Client() *Client {
	conn, err := grpc.Dial("fake", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return c.listener.Dial()
	}))
	if err != nil {
		panic(err)
	}

	return &Client{conn: conn, KV: &kv{remote: regattapb.NewKVClient(conn)}}
}

func convKeyValuesToProto(in []*KeyValue) (out []*regattapb.KeyValue) {
	for _, t := range in {
		out = append(out, (*regattapb.KeyValue)(t))
	}
	return
}

func convResponsesToProto(in []*ResponseOp) (out []*regattapb.ResponseOp) {
	for _, t := range in {
		out = append(out, (*regattapb.ResponseOp)(t))
	}
	return
}

func convToHeaderProto(rh *ResponseHeader) *regattapb.ResponseHeader {
	return &regattapb.ResponseHeader{
		ShardId:      rh.ShardId,
		ReplicaId:    rh.ReplicaId,
		Revision:     rh.Revision,
		RaftTerm:     rh.RaftTerm,
		RaftLeaderId: rh.RaftLeaderId,
	}
}
