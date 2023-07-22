// Copyright JAMF Software, LLC

package client

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// unaryClientInterceptor returns a new retrying unary client interceptor.
//
// The default configuration of the interceptor is to not retry *at all*. This behaviour can be
// changed through options (e.g. WithMax) on creation of the interceptor or on call (through grpc.CallOptions).
func (c *Client) unaryClientInterceptor(optFuncs ...retryOption) grpc.UnaryClientInterceptor {
	intOpts := reuseOrNewWithCallOptions(defaultOptions, optFuncs)
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = withVersion(ctx)
		grpcOpts, retryOpts := filterCallOptions(opts)
		callOpts := reuseOrNewWithCallOptions(intOpts, retryOpts)
		// short circuit for simplicity, and avoiding allocations.
		if callOpts.max == 0 {
			return invoker(ctx, method, req, reply, cc, grpcOpts...)
		}
		var lastErr error
		for attempt := uint(0); attempt < callOpts.max; attempt++ {
			if err := waitRetryBackoff(ctx, attempt, callOpts); err != nil {
				return err
			}
			c.lg.Debugf("retrying of unary invoker target=%s method=%s attempt=%d", cc.Target(), method, attempt)
			lastErr = invoker(ctx, method, req, reply, cc, grpcOpts...)
			if lastErr == nil {
				return nil
			}
			c.lg.Warnf("retrying of unary invoker failed target=%s method=%s attempt=%d %v", cc.Target(), method, attempt, lastErr)
			if isContextError(lastErr) {
				if ctx.Err() != nil {
					// its the context deadline or cancellation.
					return lastErr
				}
				continue
			}
			if !isSafeRetry(c, lastErr, callOpts) {
				return lastErr
			}
		}
		return lastErr
	}
}

// streamClientInterceptor returns a new retrying stream client interceptor for server side streaming calls.
//
// The default configuration of the interceptor is to not retry *at all*. This behaviour can be
// changed through options (e.g. WithMax) on creation of the interceptor or on call (through grpc.CallOptions).
//
// Retry logic is available *only for ServerStreams*, i.e. 1:n streams, as the internal logic needs
// to buffer the messages sent by the client. If retry is enabled on any other streams (ClientStreams,
// BidiStreams), the retry interceptor will fail the call.
func (c *Client) streamClientInterceptor(optFuncs ...retryOption) grpc.StreamClientInterceptor {
	intOpts := reuseOrNewWithCallOptions(defaultOptions, optFuncs)
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = withVersion(ctx)
		grpcOpts, retryOpts := filterCallOptions(opts)
		callOpts := reuseOrNewWithCallOptions(intOpts, retryOpts)
		// short circuit for simplicity, and avoiding allocations.
		if callOpts.max == 0 {
			return streamer(ctx, desc, cc, method, grpcOpts...)
		}
		if desc.ClientStreams {
			return nil, status.Errorf(codes.Unimplemented, "client/retry_interceptor: cannot retry on ClientStreams, set Disable()")
		}
		newStreamer, err := streamer(ctx, desc, cc, method, grpcOpts...)
		if err != nil {
			c.lg.Errorf("streamer failed to create ClientStream %v", err)
			return nil, err // TODO(mwitkow): Maybe dial and transport errors should be retriable?
		}
		retryingStreamer := &serverStreamingRetryingStream{
			client:       c,
			ClientStream: newStreamer,
			callOpts:     callOpts,
			ctx:          ctx,
			streamerCall: func(ctx context.Context) (grpc.ClientStream, error) {
				return streamer(ctx, desc, cc, method, grpcOpts...)
			},
		}
		return retryingStreamer, nil
	}
}

// type serverStreamingRetryingStream is the implementation of grpc.ClientStream that acts as a
// proxy to the underlying call. If any of the RecvMsg() calls fail, it will try to reestablish
// a new ClientStream according to the retry policy.
type serverStreamingRetryingStream struct {
	grpc.ClientStream
	client        *Client
	bufferedSends []interface{} // single message that the client can sen
	receivedGood  bool          // indicates whether any prior receives were successful
	wasClosedSend bool          // indicates that CloseSend was closed
	ctx           context.Context
	callOpts      *options
	streamerCall  func(ctx context.Context) (grpc.ClientStream, error)
	mu            sync.RWMutex
}

func (s *serverStreamingRetryingStream) setStream(clientStream grpc.ClientStream) {
	s.mu.Lock()
	s.ClientStream = clientStream
	s.mu.Unlock()
}

func (s *serverStreamingRetryingStream) getStream() grpc.ClientStream {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ClientStream
}

func (s *serverStreamingRetryingStream) SendMsg(m interface{}) error {
	s.mu.Lock()
	s.bufferedSends = append(s.bufferedSends, m)
	s.mu.Unlock()
	return s.getStream().SendMsg(m)
}

func (s *serverStreamingRetryingStream) CloseSend() error {
	s.mu.Lock()
	s.wasClosedSend = true
	s.mu.Unlock()
	return s.getStream().CloseSend()
}

func (s *serverStreamingRetryingStream) Header() (metadata.MD, error) {
	return s.getStream().Header()
}

func (s *serverStreamingRetryingStream) Trailer() metadata.MD {
	return s.getStream().Trailer()
}

func (s *serverStreamingRetryingStream) RecvMsg(m interface{}) error {
	attemptRetry, lastErr := s.receiveMsgAndIndicateRetry(m)
	if !attemptRetry {
		return lastErr // success or hard failure
	}

	// We start off from attempt 1, because zeroth was already made on normal SendMsg().
	for attempt := uint(1); attempt < s.callOpts.max; attempt++ {
		if err := waitRetryBackoff(s.ctx, attempt, s.callOpts); err != nil {
			return err
		}
		newStream, err := s.reestablishStreamAndResendBuffer(s.ctx)
		if err != nil {
			s.client.lg.Errorf("failed reestablishStreamAndResendBuffer %v", err)
			return err
		}
		s.setStream(newStream)

		s.client.lg.Warnf("retrying RecvMsg %v", lastErr)
		attemptRetry, lastErr = s.receiveMsgAndIndicateRetry(m)
		if !attemptRetry {
			return lastErr
		}
	}
	return lastErr
}

func (s *serverStreamingRetryingStream) receiveMsgAndIndicateRetry(m interface{}) (bool, error) {
	s.mu.RLock()
	wasGood := s.receivedGood
	s.mu.RUnlock()
	err := s.getStream().RecvMsg(m)
	if err == nil || err == io.EOF {
		s.mu.Lock()
		s.receivedGood = true
		s.mu.Unlock()
		return false, err
	} else if wasGood {
		// previous RecvMsg in the stream succeeded, no retry logic should interfere
		return false, err
	}
	if isContextError(err) {
		if s.ctx.Err() != nil {
			return false, err
		}
		// its the callCtx deadline or cancellation, in which case try again.
		return true, err
	}
	return isSafeRetry(s.client, err, s.callOpts), err
}

func (s *serverStreamingRetryingStream) reestablishStreamAndResendBuffer(callCtx context.Context) (grpc.ClientStream, error) {
	s.mu.RLock()
	bufferedSends := s.bufferedSends
	s.mu.RUnlock()
	newStream, err := s.streamerCall(callCtx)
	if err != nil {
		return nil, err
	}
	for _, msg := range bufferedSends {
		if err := newStream.SendMsg(msg); err != nil {
			return nil, err
		}
	}
	if err := newStream.CloseSend(); err != nil {
		return nil, err
	}
	return newStream, nil
}

func waitRetryBackoff(ctx context.Context, attempt uint, callOpts *options) error {
	waitTime := time.Duration(0)
	if attempt > 0 {
		waitTime = callOpts.backoffFunc(attempt)
	}
	if waitTime > 0 {
		timer := time.NewTimer(waitTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			return contextErrToGrpcErr(ctx.Err())
		case <-timer.C:
		}
	}
	return nil
}

// isSafeRetry returns "true", if request is safe for retry with the given error.
func isSafeRetry(c *Client, err error, callOpts *options) bool {
	if isContextError(err) {
		return false
	}

	switch callOpts.retryPolicy {
	case repeatable:
		return isSafeRetryImmutableRPC(err)
	case nonRepeatable:
		return isSafeRetryMutableRPC(err)
	default:
		c.lg.Warn("unrecognized retry policy retryPolicy=%s", callOpts.retryPolicy.String())
		return false
	}
}

func isContextError(err error) bool {
	return status.Code(err) == codes.DeadlineExceeded || status.Code(err) == codes.Canceled
}

func contextErrToGrpcErr(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return status.Errorf(codes.DeadlineExceeded, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Errorf(codes.Canceled, err.Error())
	default:
		return status.Errorf(codes.Unknown, err.Error())
	}
}

func reuseOrNewWithCallOptions(opt *options, retryOptions []retryOption) *options {
	if len(retryOptions) == 0 {
		return opt
	}
	optCopy := &options{}
	*optCopy = *opt
	for _, f := range retryOptions {
		f.applyFunc(optCopy)
	}
	return optCopy
}

func filterCallOptions(callOptions []grpc.CallOption) (grpcOptions []grpc.CallOption, retryOptions []retryOption) {
	for _, opt := range callOptions {
		if co, ok := opt.(retryOption); ok {
			retryOptions = append(retryOptions, co)
		} else {
			grpcOptions = append(grpcOptions, opt)
		}
	}
	return grpcOptions, retryOptions
}

// isSafeRetryImmutableRPC returns "true" when an immutable request is safe for retry.
//
// immutable requests (e.g. Get) should be retried unless it's
// an obvious server-side error (e.g. rpctypes.ErrRequestTooLarge).
//
// Returning "false" means retry should stop, since client cannot
// handle itself even with retries.
func isSafeRetryImmutableRPC(err error) bool {
	// only retry if unavailable
	ev, ok := status.FromError(err)
	if !ok {
		// all errors from RPC is typed "grpc/status.(*statusError)"
		// (ref. https://github.com/grpc/grpc-go/pull/1782)
		//
		// if the error type is not "grpc/status.(*statusError)",
		// it could be from "Dial"
		// TODO: do not retry for now
		// ref. https://github.com/grpc/grpc-go/issues/1581
		return false
	}
	return ev.Code() == codes.Unavailable
}

// isSafeRetryMutableRPC returns "true" when a mutable request is safe for retry.
//
// mutable requests (e.g. Put, Delete, Txn) should only be retried
// when the status code is codes.Unavailable when initial connection
// has not been established (no endpoint is up).
//
// Returning "false" means retry should stop, otherwise it violates
// write-at-most-once semantics.
func isSafeRetryMutableRPC(err error) bool {
	if ev, ok := status.FromError(err); ok && ev.Code() != codes.Unavailable {
		// not safe for mutable RPCs
		// e.g. interrupted by non-transient error that client cannot handle itself,
		// or transient error while the connection has already been established
		return false
	}
	desc := ErrorDesc(err)
	return desc == "there is no address available" || desc == "there is no connection available"
}

func ErrorDesc(err error) string {
	if s, ok := status.FromError(err); ok {
		return s.Message()
	}
	return err.Error()
}
