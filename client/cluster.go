// Copyright JAMF Software, LLC

package client

import (
	"context"
	"fmt"

	pb "github.com/jamf/regatta-go/proto"
	"google.golang.org/grpc"
)

type (
	Member             pb.Member
	MemberListResponse pb.MemberListResponse
	StatusResponse     pb.StatusResponse
)

type Cluster interface {
	// MemberList lists the current cluster membership.
	MemberList(ctx context.Context, opts ...OpOption) (*MemberListResponse, error)
	// Status gets the status of the endpoint.
	Status(ctx context.Context, endpoint string) (*StatusResponse, error)
}

type cluster struct {
	remote   pb.ClusterClient
	dial     func(endpoint string) (pb.ClusterClient, func(), error)
	callOpts []grpc.CallOption
}

func NewCluster(c *Client) Cluster {
	api := &cluster{
		remote: &retryClusterClient{cc: pb.NewClusterClient(c.conn)},
		dial: func(endpoint string) (pb.ClusterClient, func(), error) {
			conn, err := c.Dial(endpoint)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to dial endpoint %s with maintenance client: %v", endpoint, err)
			}

			cancel := func() { conn.Close() }
			return &retryClusterClient{cc: pb.NewClusterClient(c.conn)}, cancel, nil
		},
	}
	if c != nil {
		api.callOpts = c.callOpts
	}
	return api
}

func NewClusterFromClusterClient(remote pb.ClusterClient, c *Client) Cluster {
	api := &cluster{remote: remote}
	if c != nil {
		api.callOpts = c.callOpts
	}
	return api
}

func (c *cluster) MemberList(ctx context.Context, opts ...OpOption) (*MemberListResponse, error) {
	opt := OpGet("", opts...)
	resp, err := c.remote.MemberList(ctx, &pb.MemberListRequest{Linearizable: !opt.serializable}, c.callOpts...)
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
	resp, err := remote.Status(ctx, &pb.StatusRequest{}, c.callOpts...)
	if err != nil {
		return nil, toErr(ctx, err)
	}
	return (*StatusResponse)(resp), nil
}

type retryClusterClient struct {
	cc pb.ClusterClient
}

func (r *retryClusterClient) MemberList(ctx context.Context, in *pb.MemberListRequest, opts ...grpc.CallOption) (resp *pb.MemberListResponse, err error) {
	return r.cc.MemberList(ctx, in, append(opts, withRetryPolicy(repeatable))...)
}

func (r *retryClusterClient) Status(ctx context.Context, in *pb.StatusRequest, opts ...grpc.CallOption) (*pb.StatusResponse, error) {
	return r.cc.Status(ctx, in, append(opts, withRetryPolicy(repeatable))...)
}
