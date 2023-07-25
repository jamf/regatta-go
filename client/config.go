// Copyright JAMF Software, LLC

package client

import (
	"context"
	"crypto/tls"
	"time"

	"google.golang.org/grpc"
)

type Config struct {
	// Endpoints is a list of URLs.
	Endpoints []string `json:"endpoints"`

	// AutoSyncInterval is the interval to update endpoints with its latest members.
	// 0 disables auto-sync. By default, auto-sync is disabled.
	AutoSyncInterval time.Duration `json:"auto-sync-interval"`

	// DialTimeout is the timeout for failing to establish a connection.
	DialTimeout time.Duration `json:"dial-timeout"`

	// DialKeepAliveTime is the time after which client pings the server to see if
	// transport is alive.
	DialKeepAliveTime time.Duration `json:"dial-keep-alive-time"`

	// DialKeepAliveTimeout is the time that the client waits for a response for the
	// keep-alive probe. If the response is not received in this time, the connection is closed.
	DialKeepAliveTimeout time.Duration `json:"dial-keep-alive-timeout"`

	// MaxCallSendMsgSize is the client-side request send limit in bytes.
	// If 0, it defaults to 2.0 MiB (2 * 1024 * 1024).
	// Make sure that "MaxCallSendMsgSize" < server-side default send/recv limit.
	// ("--max-request-bytes" flag to regatta or "embed.Config.MaxRequestBytes").
	MaxCallSendMsgSize int

	// MaxCallRecvMsgSize is the client-side response receive limit.
	// If 0, it defaults to "math.MaxInt32", because range response can
	// easily exceed request send limits.
	// Make sure that "MaxCallRecvMsgSize" >= server-side default send/recv limit.
	// ("--max-recv-bytes" flag to regatta).
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

	// Logger sets client-side logger.
	// If nil, fallback to building LogConfig.
	Logger Logger

	// PermitWithoutStream when set will allow client to send keepalive pings to server without any active streams(RPCs).
	PermitWithoutStream bool `json:"permit-without-stream"`

	// RejectOldCluster when set will refuse to create a client against an outdated cluster.
	RejectOldCluster bool `json:"reject-old-cluster"`
}

// ConfigSpec is the configuration from users, which comes from command-line flags,
// environment variables or config file. It is a fully declarative configuration,
// and can be serialized & deserialized to/from JSON.
type ConfigSpec struct {
	Logger
	Endpoints        []string      `json:"endpoints"`
	DialTimeout      time.Duration `json:"dial-timeout"`
	KeepAliveTime    time.Duration `json:"keepalive-time"`
	KeepAliveTimeout time.Duration `json:"keepalive-timeout"`
	Secure           *SecureConfig `json:"secure"`
}

type SecureConfig struct {
	Cert       string `json:"cert"`
	Key        string `json:"key"`
	Cacert     string `json:"cacert"`
	ServerName string `json:"server-name"`

	InsecureTransport  bool `json:"insecure-transport"`
	InsecureSkipVerify bool `json:"insecure-skip-tls-verify"`
}

// NewClientConfig creates a Config based on the provided ConfigSpec.
func NewClientConfig(confSpec *ConfigSpec) (*Config, error) {
	if confSpec.Logger == nil {
		confSpec.Logger = defaultLogger
	}
	tlsCfg, err := newTLSConfig(confSpec.Secure, confSpec)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Endpoints:            confSpec.Endpoints,
		DialTimeout:          confSpec.DialTimeout,
		DialKeepAliveTime:    confSpec.KeepAliveTime,
		DialKeepAliveTimeout: confSpec.KeepAliveTimeout,
		TLS:                  tlsCfg,
		Logger:               confSpec,
	}

	return cfg, nil
}

func newTLSConfig(scfg *SecureConfig, lg Logger) (*tls.Config, error) {
	var (
		tlsCfg *tls.Config
		err    error
	)

	if scfg == nil {
		return nil, nil
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
			return nil, err
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

	return tlsCfg, nil
}
