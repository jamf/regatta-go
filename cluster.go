// Copyright JAMF Software, LLC

package client

import (
	"context"
	"fmt"

	"github.com/jamf/regatta-go/internal/proto"
	"google.golang.org/grpc"
)

type (
	Member             regattapb.Member
	MemberListResponse regattapb.MemberListResponse
	TableStatus        regattapb.TableStatus
	// StatusResponse represents response from Status API.
	StatusResponse struct {
		// Id is the member ID of this member.
		Id string
		// Version is the semver version used by the responding member.
		Version string
		// Info is the additional server info.
		Info string
		// Tables is a status of tables of the responding member.
		Tables map[string]*TableStatus
		// Errors contains alarm/health information and status.
		Errors []string
		// Config contains the member configuration.
		Config map[string]any
	}
)

type Cluster interface {
	// MemberList lists the current cluster membership.
	MemberList(ctx context.Context) (*MemberListResponse, error)
	// Status gets the status of the endpoint.
	Status(ctx context.Context, endpoint string) (*StatusResponse, error)
}

type cluster struct {
	remote   regattapb.ClusterClient
	dial     func(endpoint string) (regattapb.ClusterClient, func(), error)
	callOpts []grpc.CallOption
}

func newCluster(c *Client) Cluster {
	api := &cluster{
		remote:   &retryClusterClient{cc: regattapb.NewClusterClient(c.conn)},
		callOpts: c.callOpts,
		dial: func(endpoint string) (regattapb.ClusterClient, func(), error) {
			conn, err := c.Dial(endpoint)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to dial endpoint %s with maintenance client: %v", endpoint, err)
			}

			cancel := func() { conn.Close() }
			return &retryClusterClient{cc: regattapb.NewClusterClient(c.conn)}, cancel, nil
		},
	}
	return api
}

func NewClusterFromClusterClient(remote regattapb.ClusterClient, c *Client) Cluster {
	api := &cluster{remote: remote}
	if c != nil {
		api.callOpts = c.callOpts
	}
	return api
}

func (c *cluster) MemberList(ctx context.Context) (*MemberListResponse, error) {
	resp, err := c.remote.MemberList(ctx, &regattapb.MemberListRequest{}, append(c.callOpts, withRepeatable())...)
	if err == nil {
		return (*MemberListResponse)(resp), nil
	}
	return nil, toErr(ctx, err)
}

func (c *cluster) Status(ctx context.Context, endpoint string) (*StatusResponse, error) {
	remote, cancel, err := c.dial(endpoint)
	if err != nil {
		return nil, toErr(ctx, err)
	}
	defer cancel()
	resp, err := remote.Status(ctx, &regattapb.StatusRequest{Config: true}, append(c.callOpts, withRepeatable())...)
	if err != nil {
		return nil, toErr(ctx, err)
	}
	return mapProtoStatusResponse(resp), nil
}

func mapProtoStatusResponse(resp *regattapb.StatusResponse) *StatusResponse {
	ts := make(map[string]*TableStatus, len(resp.Tables))

	for k, v := range resp.Tables {
		ts[k] = (*TableStatus)(v)
	}

	return &StatusResponse{
		Id:      resp.Id,
		Version: resp.Version,
		Info:    resp.Info,
		Tables:  ts,
		Errors:  resp.Errors,
		Config:  resp.Config.AsMap(),
	}
}

type retryClusterClient struct {
	cc regattapb.ClusterClient
}

func (r *retryClusterClient) MemberList(ctx context.Context, in *regattapb.MemberListRequest, opts ...grpc.CallOption) (resp *regattapb.MemberListResponse, err error) {
	return r.cc.MemberList(ctx, in, append(opts, withRepeatable())...)
}

func (r *retryClusterClient) Status(ctx context.Context, in *regattapb.StatusRequest, opts ...grpc.CallOption) (*regattapb.StatusResponse, error) {
	return r.cc.Status(ctx, in, append(opts, withRepeatable())...)
}
