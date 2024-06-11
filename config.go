// Copyright JAMF Software, LLC

package client

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"google.golang.org/grpc"
)

type config struct {
	// Endpoints is a list of URLs.
	Endpoints []string

	// AutoSyncInterval is the interval to update endpoints with its latest members.
	// 0 disables auto-sync. By default, auto-sync is disabled.
	AutoSyncInterval time.Duration

	// DialKeepAliveTime is the time after which client pings the server to see if
	// transport is alive.
	DialKeepAliveTime time.Duration

	// DialKeepAliveTimeout is the time that the client waits for a response for the
	// keep-alive probe. If the response is not received in this time, the connection is closed.
	DialKeepAliveTimeout time.Duration

	// InitialConnectionTimeout is the timeout for establishing first connection to the server when the client is instantiated.
	InitialConnectionTimeout time.Duration

	// MaxCallSendMsgSize is the client-side request send limit in bytes.
	// If 0, it defaults to 2.0 MiB (2 * 1024 * 1024).
	// Make sure that "MaxCallSendMsgSize" < server-side default send/recv limit.
	// ("--api.max-request-bytes" flag to regatta or "embed.Config.MaxRequestBytes").
	MaxCallSendMsgSize int

	// MaxCallRecvMsgSize is the client-side response receive limit.
	// If 0, it defaults to "math.MaxInt32", because range response can
	// easily exceed request send limits.
	// Make sure that "MaxCallRecvMsgSize" >= server-side default send/recv limit.
	// ("--api.max-recv-bytes" flag to regatta).
	MaxCallRecvMsgSize int

	// TLS holds the client secure credentials, if any.
	TLS *tls.Config

	// DialOptions is a list of dial options for the grpc client (e.g., for interceptors).
	// For example, pass "grpc.WithBlock()" to block until the underlying connection is up.
	// Without this, Dial returns immediately and connecting the server happens in background.
	DialOptions []grpc.DialOption

	// Context is the default client context; it can be used to cancel grpc dial out and
	// other operations that do not have an explicit context.
	Context context.Context

	// Logger sets client-side logger. If nil, fallback to building LogConfig.
	Logger Logger

	// PermitWithoutStream when set will allow client to send keepalive pings to server without any active streams(RPCs).
	PermitWithoutStream bool

	// RejectOldCluster when set will refuse to create a client against an outdated cluster.
	RejectOldCluster bool

	Hooks []Hook
}

type SecureConfig struct {
	Cert       string `json:"cert"`
	Key        string `json:"key"`
	Cacert     string `json:"cacert"`
	ServerName string `json:"server-name"`

	InsecureTransport  bool `json:"insecure-transport"`
	InsecureSkipVerify bool `json:"insecure-skip-tls-verify"`
}

func newTLSConfig(scfg *SecureConfig, lg Logger) *tls.Config {
	var (
		tlsCfg *tls.Config
		err    error
	)

	if scfg == nil {
		return nil
	}

	if scfg.Cert != "" || scfg.Key != "" || scfg.Cacert != "" || scfg.ServerName != "" {
		cfgtls := &TLSInfo{
			CertFile:      scfg.Cert,
			KeyFile:       scfg.Key,
			TrustedCAFile: scfg.Cacert,
			ServerName:    scfg.ServerName,
			Logger:        lg,
		}
		if tlsCfg, err = cfgtls.ClientConfig(); err != nil {
			panic(err)
		}
	}

	// If key/cert is not given but user wants secure connection, we
	// should still setup an empty tls configuration for gRPC to setup
	// secure connection.
	if tlsCfg == nil && !scfg.InsecureTransport {
		tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13}
	}

	// If the user wants to skip TLS verification then we should set
	// the InsecureSkipVerify flag in tls configuration.
	if scfg.InsecureSkipVerify {
		if tlsCfg == nil {
			tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13}
		}
		tlsCfg.InsecureSkipVerify = scfg.InsecureSkipVerify
	}

	return tlsCfg
}

func processHooks(hooks []Hook) ([]Hook, error) {
	var processedHooks []Hook
	for _, hook := range hooks {
		if implementsAnyHook(hook) {
			processedHooks = append(processedHooks, hook)
		} else if moreHooks, ok := hook.([]Hook); ok {
			more, err := processHooks(moreHooks)
			if err != nil {
				return nil, err
			}
			processedHooks = append(processedHooks, more...)
		} else {
			return nil, errors.New("found an argument that implements no hook interfaces")
		}
	}
	return processedHooks, nil
}

// Option is a function type that can be passed as argument to New to configure client.
type Option func(config *config)

// WithDialOptions sets the list of dial options for the grpc client (e.g., for interceptors).
// For example, pass "grpc.WithBlock()" to block until the underlying connection is up.
// Without this, Dial returns immediately and connecting the server happens in background.
func WithDialOptions(dial ...grpc.DialOption) Option {
	return func(config *config) {
		config.DialOptions = dial
	}
}

// WithEndpoints sets the list of seed URLs.
func WithEndpoints(endpoints ...string) Option {
	return func(config *config) {
		config.Endpoints = endpoints
	}
}

// WithDialTimeout sets the dial timeout.
// Deprecated: use WithInitialConnectionTimeout instead.
func WithDialTimeout(t time.Duration) Option {
	return func(config *config) {
		config.InitialConnectionTimeout = t
	}
}

// WithInitialConnectionTimeout sets the initial connection timeout.
func WithInitialConnectionTimeout(t time.Duration) Option {
	return func(config *config) {
		config.InitialConnectionTimeout = t
	}
}

// WithKeepalive sets the keepalive protocol timeouts, keepaliveTime is the time after which client pings the server to see if
// transport is alive. keepaliveTimeout is the time that the client waits for a response for the keep-alive probe.
// If the response is not received in this time, the connection is closed.
func WithKeepalive(keepaliveTime, keepaliveTimeout time.Duration) Option {
	return func(config *config) {
		config.DialKeepAliveTime = keepaliveTime
		config.DialKeepAliveTimeout = keepaliveTimeout
	}
}

// WithAutoSyncInterval is the interval to update endpoints with its latest members.
// 0 disables auto-sync. By default, auto-sync is disabled.
func WithAutoSyncInterval(interval time.Duration) Option {
	return func(config *config) {
		config.AutoSyncInterval = interval
	}
}

// WithLogger sets client-side logger. Defaults to NOOp logger.
func WithLogger(logger Logger) Option {
	return func(config *config) {
		config.Logger = logger
	}
}

// WithHooks sets plugin hooks, check the plugin directory for available plugin modules.
func WithHooks(hooks ...Hook) Option {
	return func(config *config) {
		config.Hooks = hooks
	}
}

// WithSecureConfig sets the TLS and related configs.
func WithSecureConfig(sc *SecureConfig) Option {
	return func(config *config) {
		config.TLS = newTLSConfig(sc, config.Logger)
	}
}

// WithMaxCallRecvMsgSize sets the client-side response receive limit.
// If 0, it defaults to "math.MaxInt32", because range response can
// easily exceed request send limits.
// Make sure that "MaxCallRecvMsgSize" >= server-side default send/recv limit.
// ("--api.max-recv-bytes" flag to regatta).
func WithMaxCallRecvMsgSize(max int) Option {
	return func(config *config) {
		config.MaxCallRecvMsgSize = max
	}
}

// WithMaxCallSendMsgSize sets the client-side request send limit in bytes.
// If 0, it defaults to 2.0 MiB (2 * 1024 * 1024).
// Make sure that "MaxCallSendMsgSize" < server-side default send/recv limit.
// ("--api.max-send-bytes" flag to regatta or "embed.Config.MaxRequestBytes").
func WithMaxCallSendMsgSize(max int) Option {
	return func(config *config) {
		config.MaxCallSendMsgSize = max
	}
}

// WithRejectOldCluster set will refuse to create a client against an outdated cluster.
func WithRejectOldCluster(roc bool) Option {
	return func(config *config) {
		config.RejectOldCluster = roc
	}
}
