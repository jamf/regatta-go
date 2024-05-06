// Copyright JAMF Software, LLC

package client

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jamf/regatta-go/credentials"
	"github.com/jamf/regatta-go/internal/endpoint"
	"github.com/jamf/regatta-go/internal/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpccredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

var (
	ErrNoAvailableEndpoints = errors.New("regattaclient: no available endpoints")
	ErrOldCluster           = errors.New("regattaclient: old cluster version")
)

var Version = "unknown"

// Client provides and manages an regatta client session.
type Client struct {
	KV
	Cluster
	Tables

	conn *grpc.ClientConn

	cfg      config
	creds    grpccredentials.TransportCredentials
	resolver *resolver.RegattaManualResolver

	epMu      sync.RWMutex
	endpoints []string

	ctx    context.Context
	cancel context.CancelFunc

	authTokenBundle credentials.Bundle

	callOpts []grpc.CallOption

	lgMu sync.RWMutex
	lg   Logger
}

// New creates a new regatta client from a given configuration.
func New(opts ...Option) (*Client, error) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	return newClient(cfg)
}

// NewFromURL creates a new regatta client from a URL.
func NewFromURL(url string, opts ...Option) (*Client, error) {
	cfg := &config{Endpoints: []string{url}}
	for _, opt := range opts {
		opt(cfg)
	}
	return newClient(cfg)
}

// NewFromURLs creates a new regatta client from URLs.
func NewFromURLs(urls []string, opts ...Option) (*Client, error) {
	cfg := &config{Endpoints: urls}
	for _, opt := range opts {
		opt(cfg)
	}
	return newClient(cfg)
}

// SetLogger overrides the logger.
// Does not change grpcLogger, that can be explicitly configured
// using grpc_zap.ReplaceGrpcLoggerV2(..) method.
func (c *Client) SetLogger(lg Logger) *Client {
	c.lgMu.Lock()
	c.lg = lg
	c.lgMu.Unlock()
	return c
}

// Close shuts down the client's regatta connections.
func (c *Client) Close() error {
	defer func() {
		callHooks(c.cfg.Hooks, func(h HookClientClosed) {
			h.OnClientClosed(c)
		})
	}()
	c.cancel()
	if c.conn != nil {
		return toErr(c.ctx, c.conn.Close())
	}
	return c.ctx.Err()
}

// Ctx is a context for "out of band" messages (e.g., for sending
// "clean up" message when another context is canceled). It is
// canceled on client Close().
func (c *Client) Ctx() context.Context { return c.ctx }

// Endpoints lists the registered endpoints for the client.
func (c *Client) Endpoints() []string {
	// copy the slice; protect original endpoints from being changed
	c.epMu.RLock()
	defer c.epMu.RUnlock()
	eps := make([]string, len(c.endpoints))
	copy(eps, c.endpoints)
	return eps
}

// SetEndpoints updates client's endpoints.
func (c *Client) SetEndpoints(eps ...string) {
	c.epMu.Lock()
	defer c.epMu.Unlock()
	c.endpoints = eps

	c.resolver.SetEndpoints(eps)
}

// Sync synchronizes client's endpoints with the known endpoints from the regatta membership.
func (c *Client) Sync(ctx context.Context) error {
	mresp, err := c.MemberList(ctx)
	if err != nil {
		return err
	}
	var eps []string
	for _, m := range mresp.Members {
		if len(m.Name) != 0 {
			eps = append(eps, m.ClientURLs...)
		}
	}
	c.SetEndpoints(eps...)
	c.lg.Debugf("set regatta endpoints by autoSync %v", eps)
	return nil
}

func (c *Client) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	return ctx
}

func (c *Client) HandleRPC(ctx context.Context, stats stats.RPCStats) {
	callHooks(c.cfg.Hooks, func(h HookHandleRPC) {
		h.OnHandleRPC(ctx, stats)
	})
}

func (c *Client) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	return ctx
}

func (c *Client) HandleConn(ctx context.Context, stats stats.ConnStats) {
	callHooks(c.cfg.Hooks, func(h HookHandleConn) {
		h.OnHandleConn(ctx, stats)
	})
}

func (c *Client) autoSync() {
	if c.cfg.AutoSyncInterval == time.Duration(0) {
		return
	}

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(c.cfg.AutoSyncInterval):
			ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
			err := c.Sync(ctx)
			cancel()
			if err != nil && !errors.Is(err, c.ctx.Err()) {
				c.lg.Infof("auto sync endpoints failed: %v", err)
			}
		}
	}
}

// dialSetupOpts gives the dial opts prior to any authentication.
func (c *Client) dialSetupOpts(creds grpccredentials.TransportCredentials, dopts ...grpc.DialOption) (opts []grpc.DialOption) {
	if c.cfg.DialKeepAliveTime > 0 {
		params := keepalive.ClientParameters{
			Time:                c.cfg.DialKeepAliveTime,
			Timeout:             c.cfg.DialKeepAliveTimeout,
			PermitWithoutStream: c.cfg.PermitWithoutStream,
		}
		opts = append(opts, grpc.WithKeepaliveParams(params))
	}
	opts = append(opts, dopts...)

	if creds != nil {
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Interceptor retry and backoff.
	rrBackoff := withBackoff(c.roundRobinQuorumBackoff(defaultBackoffWaitBetween, defaultBackoffJitterFraction))
	opts = append(opts,
		// Disable stream retry by default since go-grpc-middleware/retry does not support client streams.
		// Streams that are safe to retry are enabled individually.
		grpc.WithStreamInterceptor(c.streamClientInterceptor(withMax(0), rrBackoff)),
		grpc.WithUnaryInterceptor(c.unaryClientInterceptor(withMax(defaultUnaryMaxRetries), rrBackoff)),
	)

	return opts
}

// Dial connects to a single endpoint using the client's config.
func (c *Client) Dial(ep string) (*grpc.ClientConn, error) {
	creds := c.credentialsForEndpoint(ep)

	// Using ad-hoc created resolver, to guarantee only explicitly given
	// endpoint is used.
	return c.dial(creds, grpc.WithResolvers(resolver.New(ep)))
}

// dialWithBalancer dials the client's current load balanced resolver group.  The scheme of the host
// of the provided endpoint determines the scheme used for all endpoints of the client connection.
func (c *Client) dialWithBalancer(dopts ...grpc.DialOption) (*grpc.ClientConn, error) {
	creds := c.credentialsForEndpoint(c.Endpoints()[0])
	opts := append(dopts, grpc.WithStatsHandler(c), grpc.WithResolvers(c.resolver))
	return c.dial(creds, opts...)
}

// dial configures and dials any grpc balancer target.
func (c *Client) dial(creds grpccredentials.TransportCredentials, dopts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts := c.dialSetupOpts(creds, dopts...)
	if c.authTokenBundle != nil {
		opts = append(opts, grpc.WithPerRPCCredentials(c.authTokenBundle.PerRPCCredentials()))
	}

	opts = append(opts, c.cfg.DialOptions...)

	dctx := c.ctx
	if c.cfg.DialTimeout > 0 {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(c.ctx, c.cfg.DialTimeout)
		defer cancel() // TODO: Is this right for cases where grpc.WithBlock() is not set on the dial options?
	}
	target := fmt.Sprintf("%s://%p/%s", resolver.Schema, c, authority(c.endpoints[0]))
	conn, err := grpc.DialContext(dctx, target, opts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func authority(endpoint string) string {
	spl := strings.SplitN(endpoint, "://", 2)
	if len(spl) < 2 {
		if strings.HasPrefix(endpoint, "unix:") {
			return endpoint[len("unix:"):]
		}
		if strings.HasPrefix(endpoint, "unixs:") {
			return endpoint[len("unixs:"):]
		}
		return endpoint
	}
	return spl[1]
}

func (c *Client) credentialsForEndpoint(ep string) grpccredentials.TransportCredentials {
	r := endpoint.RequiresCredentials(ep)
	switch r {
	case endpoint.CredsDrop:
		return nil
	case endpoint.CredsOptional:
		return c.creds
	case endpoint.CredsRequire:
		if c.creds != nil {
			return c.creds
		}
		return credentials.NewBundle(credentials.Config{}).TransportCredentials()
	default:
		panic(fmt.Errorf("unsupported CredsRequirement: %v", r))
	}
}

func newClient(cfg *config) (*Client, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, ErrNoAvailableEndpoints
	}
	var creds grpccredentials.TransportCredentials
	if cfg.TLS != nil {
		creds = credentials.NewBundle(credentials.Config{TLSConfig: cfg.TLS}).TransportCredentials()
	}

	// use a temporary skeleton client to bootstrap first connection
	baseCtx := context.TODO()
	if cfg.Context != nil {
		baseCtx = cfg.Context
	}

	ctx, cancel := context.WithCancel(baseCtx)
	client := &Client{
		conn:     nil,
		cfg:      *cfg,
		creds:    creds,
		ctx:      ctx,
		cancel:   cancel,
		callOpts: defaultCallOpts,
	}

	if cfg.Logger != nil {
		client.lg = cfg.Logger
	} else {
		client.lg = defaultLogger
	}

	if cfg.MaxCallSendMsgSize > 0 || cfg.MaxCallRecvMsgSize > 0 {
		if cfg.MaxCallRecvMsgSize > 0 && cfg.MaxCallSendMsgSize > cfg.MaxCallRecvMsgSize {
			return nil, fmt.Errorf("gRPC message recv limit (%d bytes) must be greater than send limit (%d bytes)", cfg.MaxCallRecvMsgSize, cfg.MaxCallSendMsgSize)
		}
		callOpts := []grpc.CallOption{
			defaultWaitForReady,
			defaultMaxCallSendMsgSize,
			defaultMaxCallRecvMsgSize,
		}
		if cfg.MaxCallSendMsgSize > 0 {
			callOpts[1] = grpc.MaxCallSendMsgSize(cfg.MaxCallSendMsgSize)
		}
		if cfg.MaxCallRecvMsgSize > 0 {
			callOpts[2] = grpc.MaxCallRecvMsgSize(cfg.MaxCallRecvMsgSize)
		}
		client.callOpts = callOpts
	}

	processedHooks, err := processHooks(cfg.Hooks)
	if err != nil {
		return nil, err
	}
	cfg.Hooks = processedHooks

	callHooks(cfg.Hooks, func(h HookNewClient) {
		h.OnNewClient(client)
	})

	client.resolver = resolver.New(cfg.Endpoints...)

	if len(cfg.Endpoints) < 1 {
		client.cancel()
		return nil, errors.New("at least one Endpoint is required in client config")
	}
	client.SetEndpoints(cfg.Endpoints...)

	// Use a provided endpoint target so that for https:// without any tls config given, then
	// grpc will assume the certificate server name is the endpoint host.
	conn, err := client.dialWithBalancer()
	if err != nil {
		client.cancel()
		client.resolver.Close()
		// TODO: Error like `fmt.Errorf(dialing [%s] failed: %v, strings.Join(cfg.Endpoints, ";"), err)` would help with debugging a lot.
		return nil, err
	}
	client.conn = conn

	client.Cluster = newCluster(client)
	client.Tables = newTables(client)
	client.KV = newKV(client)

	if cfg.RejectOldCluster {
		if err := client.checkVersion(); err != nil {
			_ = client.Close()
			return nil, err
		}
	}

	go client.autoSync()
	return client, nil
}

// roundRobinQuorumBackoff retries against quorum between each backoff.
// This is intended for use with a round-robin load balancer.
func (c *Client) roundRobinQuorumBackoff(waitBetween time.Duration, jitterFraction float64) backoffFunc {
	return func(attempt uint) time.Duration {
		// after each round-robin across quorum, backoff for our wait between duration
		n := uint(len(c.Endpoints()))
		quorum := n/2 + 1
		if attempt%quorum == 0 {
			c.lg.Debugf("backoff attempt=%d quorum=%d waitBetween=%d jitterFraction=%d", attempt, quorum, waitBetween, jitterFraction)
			return jitterUp(waitBetween, jitterFraction)
		}
		c.lg.Debugf("backoff skipped attempt=%d quorum=%d", attempt, quorum)
		return 0
	}
}

func (c *Client) checkVersion() (err error) {
	var wg sync.WaitGroup

	eps := c.Endpoints()
	errc := make(chan error, len(eps))
	ctx, cancel := context.WithCancel(c.ctx)
	if c.cfg.DialTimeout > 0 {
		cancel()
		ctx, cancel = context.WithTimeout(c.ctx, c.cfg.DialTimeout)
	}

	wg.Add(len(eps))
	for _, ep := range eps {
		// if cluster is current, any endpoint gives a recent version
		go func(e string) {
			defer wg.Done()
			resp, rerr := c.Status(ctx, e)
			if rerr != nil {
				errc <- rerr
				return
			}
			vs := strings.Split(resp.Version, ".")
			maj, minor := 0, 0
			if len(vs) >= 2 {
				var serr error
				if maj, serr = strconv.Atoi(vs[0]); serr != nil {
					errc <- serr
					return
				}
				if minor, serr = strconv.Atoi(vs[1]); serr != nil {
					errc <- serr
					return
				}
			}
			if maj < 0 || (maj == 0 && minor < 5) {
				rerr = ErrOldCluster
			}
			errc <- rerr
		}(ep)
	}
	// wait for success
	for range eps {
		if err = <-errc; err != nil {
			break
		}
	}
	cancel()
	wg.Wait()
	return err
}

// ActiveConnection returns the current in-use connection.
func (c *Client) ActiveConnection() *grpc.ClientConn { return c.conn }

func toErr(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ev, ok := status.FromError(err); ok {
		code := ev.Code()
		switch code {
		case codes.DeadlineExceeded:
			fallthrough
		case codes.Canceled:
			if ctx.Err() != nil {
				err = ctx.Err()
			}
		}
	}
	return err
}
