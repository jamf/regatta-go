// Copyright JAMF Software, LLC

package client

import (
	"context"
	"encoding/json"

	"github.com/jamf/regatta-go/internal/proto"
	"google.golang.org/grpc"
)

type (
	CreateTableResponse regattapb.CreateTableResponse
	DeleteTableResponse regattapb.DeleteTableResponse
	ListTablesResponse  struct {
		Tables []TableInfo
	}
	TableInfo struct {
		ID     string
		Name   string
		Config map[string]any
	}
)

func (resp *CreateTableResponse) String() string {
	enc, _ := json.Marshal(resp)
	return string(enc)
}

func (resp *ListTablesResponse) String() string {
	enc, _ := json.Marshal(resp)
	return string(enc)
}

type Tables interface {
	// CreateTable creates table with specified name.
	CreateTable(ctx context.Context, name string) (*CreateTableResponse, error)
	// DeleteTable gets the status of the endpoint.
	DeleteTable(ctx context.Context, name string) (*DeleteTableResponse, error)
	// ListTables lists all currently registered tables.
	ListTables(ctx context.Context) (*ListTablesResponse, error)
}

type tables struct {
	remote   regattapb.TablesClient
	callOpts []grpc.CallOption
}

func newTables(c *Client) Tables {
	api := &tables{
		remote:   &retryTablesClient{client: regattapb.NewTablesClient(c.conn)},
		callOpts: c.callOpts,
	}
	return api
}

func NewTablesFromTablesClient(remote regattapb.TablesClient, c *Client) Tables {
	api := &tables{remote: &retryTablesClient{remote}}
	if c != nil {
		api.callOpts = c.callOpts
	}
	return api
}

func (t *tables) CreateTable(ctx context.Context, name string) (*CreateTableResponse, error) {
	resp, err := t.remote.Create(ctx, &regattapb.CreateTableRequest{Name: name}, t.callOpts...)
	if err == nil {
		return (*CreateTableResponse)(resp), nil
	}
	return nil, toErr(ctx, err)
}

func (t *tables) DeleteTable(ctx context.Context, name string) (*DeleteTableResponse, error) {
	resp, err := t.remote.Delete(ctx, &regattapb.DeleteTableRequest{Name: name}, t.callOpts...)
	if err == nil {
		return (*DeleteTableResponse)(resp), nil
	}
	return nil, toErr(ctx, err)
}

func (t *tables) ListTables(ctx context.Context) (*ListTablesResponse, error) {
	resp, err := t.remote.List(ctx, &regattapb.ListTablesRequest{}, t.callOpts...)
	if err == nil {
		return mapProtoListTablesResponse(resp), nil
	}
	return nil, toErr(ctx, err)
}

func mapProtoListTablesResponse(resp *regattapb.ListTablesResponse) *ListTablesResponse {
	ts := make([]TableInfo, len(resp.Tables))
	for k, v := range resp.Tables {
		ts[k] = TableInfo{
			ID:     v.Id,
			Name:   v.Name,
			Config: v.Config.AsMap(),
		}
	}
	return &ListTablesResponse{Tables: ts}
}

type retryTablesClient struct {
	client regattapb.TablesClient
}

func (r *retryTablesClient) Create(ctx context.Context, in *regattapb.CreateTableRequest, opts ...grpc.CallOption) (*regattapb.CreateTableResponse, error) {
	return r.client.Create(ctx, in, opts...)
}

func (r *retryTablesClient) Delete(ctx context.Context, in *regattapb.DeleteTableRequest, opts ...grpc.CallOption) (*regattapb.DeleteTableResponse, error) {
	return r.client.Delete(ctx, in, opts...)
}

func (r *retryTablesClient) List(ctx context.Context, in *regattapb.ListTablesRequest, opts ...grpc.CallOption) (*regattapb.ListTablesResponse, error) {
	return r.client.List(ctx, in, append(opts, withRepeatable())...)
}
