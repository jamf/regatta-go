// Copyright JAMF Software, LLC

package client

import (
	"context"

	"google.golang.org/grpc/stats"
)

// Hook is a hook to be called when something happens in regatta-go.
//
// The base Hook interface is useless, but wherever a hook can occur in kgo,
// the client checks if your hook implements an appropriate interface. If so,
// your hook is called.
//
// This allows you to only hook in to behavior you care about, and it allows
// the client to add more hooks in the future.
//
// All hook interfaces in this package have Hook in the name. Hooks must be
// safe for concurrent use. It is expected that hooks are fast; if a hook needs
// to take time, then copy what you need and ensure the hook is async.
type Hook any

func callHooks[T Hook](hs []Hook, fn func(T)) {
	for _, h := range hs {
		if h, ok := h.(T); ok {
			fn(h)
		}
	}
}

// HookNewClient is called in NewClient after a client is initialized. This
// hook can be used to perform final setup work in your hooks.
type HookNewClient interface {
	// OnNewClient is passed the newly initialized client, before any
	// client goroutines are started.
	OnNewClient(*Client)
}

// HookClientClosed is called in Close after a client
// has been closed. This hook can be used to perform final cleanup work.
type HookClientClosed interface {
	// OnClientClosed is passed the client that has been closed, after
	// all client-internal close cleanup has happened.
	OnClientClosed(*Client)
}

// HookHandleRPC is called whenever any RPC is made by the client.
// the provided stats could be of various types, check implementation of stats.RPCStats interface.
type HookHandleRPC interface {
	OnHandleRPC(context.Context, stats.RPCStats)
}

// HookHandleConn is called whenever connection is changed (connected/disconnected/errored).
// the provided stats could be of various types, check implementation of stats.ConnStats interface.
type HookHandleConn interface {
	OnHandleConn(context.Context, stats.ConnStats)
}

// implementsAnyHook will check the incoming Hook for any Hook implementation
func implementsAnyHook(h Hook) bool {
	switch h.(type) {
	case HookNewClient,
		HookClientClosed:
		return true
	}
	return false
}
