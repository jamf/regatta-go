// Copyright JAMF Software, LLC

package client

import (
	"math"
	"time"

	"github.com/jamf/regatta-go/client/snappy"
	"google.golang.org/grpc"
)

var (
	// client-side handling retrying of request failures where data was not written to the wire or
	// where server indicates it did not process the data. gRPC default is "WaitForReady(false)"
	// but for regatta we default to "WaitForReady(true)" to minimize client request error responses due to
	// transient failures.
	defaultWaitForReady = grpc.WaitForReady(true)

	// client-side request send limit, gRPC default is math.MaxInt32
	// Make sure that "client-side send limit < server-side default send/recv limit"
	// Same value as "embed.DefaultMaxRequestBytes" plus gRPC overhead bytes.
	defaultMaxCallSendMsgSize = grpc.MaxCallSendMsgSize(2 * 1024 * 1024)

	// client-side response receive limit, gRPC default is 4MB
	// Make sure that "client-side receive limit >= server-side default send/recv limit"
	// because range response can easily exceed request send limits
	// Default to math.MaxInt32; writes exceeding server-side send limit fails anyway.
	defaultMaxCallRecvMsgSize = grpc.MaxCallRecvMsgSize(math.MaxInt32)

	// default compression algorithm is set to snappy, compressor balance throughput latency and CPU usage.
	defaultCompressor = grpc.UseCompressor(snappy.Name)

	// client-side non-streaming retry limit, only applied to requests where server responds with
	// a error code clearly indicating it was unable to process the request such as codes.Unavailable.
	// If set to 0, retry is disabled.
	defaultUnaryMaxRetries uint = 100

	// client-side retry backoff wait between requests.
	defaultBackoffWaitBetween = 25 * time.Millisecond

	// client-side retry backoff default jitter fraction.
	defaultBackoffJitterFraction = 0.10
)

// defaultCallOpts defines a list of default "gRPC.CallOption".
// Some options are exposed to "client.Config".
// Defaults will be overridden by the settings in "client.Config".
var defaultCallOpts = []grpc.CallOption{
	defaultCompressor,
	defaultWaitForReady,
	defaultMaxCallSendMsgSize,
	defaultMaxCallRecvMsgSize,
}
