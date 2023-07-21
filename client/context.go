// Copyright JAMF Software, LLC

package client

import (
	"context"

	"google.golang.org/grpc/metadata"
)

var (
	MetadataRequireLeaderKey    = "has-leader"
	MetadataClientAPIVersionKey = "client-api-version"
	MetadataHasLeader           = "true"
)

// WithRequireLeader requires client requests to only succeed
// when the cluster has a leader.
func WithRequireLeader(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok { // no outgoing metadata ctx key, create one
		md = metadata.Pairs(MetadataRequireLeaderKey, MetadataHasLeader)
		return metadata.NewOutgoingContext(ctx, md)
	}
	copied := md.Copy() // avoid racey updates
	// overwrite/add 'hasleader' key/value
	copied.Set(MetadataRequireLeaderKey, MetadataHasLeader)
	return metadata.NewOutgoingContext(ctx, copied)
}

// withVersion embeds client version.
func withVersion(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok { // no outgoing metadata ctx key, create one
		md = metadata.Pairs(MetadataClientAPIVersionKey, Version)
		return metadata.NewOutgoingContext(ctx, md)
	}
	copied := md.Copy() // avoid racey updates
	// overwrite/add version key/value
	copied.Set(MetadataClientAPIVersionKey, Version)
	return metadata.NewOutgoingContext(ctx, copied)
}
