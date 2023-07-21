package resolver

import (
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/serviceconfig"

	"github.com/jamf/regatta-go/client/endpoint"
)

const (
	Schema = "regatta-endpoints"
)

// RegattaManualResolver is a Resolver (and resolver.Builder) that can be updated
// using SetEndpoints.
type RegattaManualResolver struct {
	*manual.Resolver
	endpoints     []string
	serviceConfig *serviceconfig.ParseResult
}

func New(endpoints ...string) *RegattaManualResolver {
	r := manual.NewBuilderWithScheme(Schema)
	return &RegattaManualResolver{Resolver: r, endpoints: endpoints, serviceConfig: nil}
}

// Build returns itself for Resolver, because it's both a builder and a resolver.
func (r *RegattaManualResolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r.serviceConfig = cc.ParseServiceConfig(`{"loadBalancingPolicy": "round_robin"}`)
	if r.serviceConfig.Err != nil {
		return nil, r.serviceConfig.Err
	}
	res, err := r.Resolver.Build(target, cc, opts)
	if err != nil {
		return nil, err
	}
	// Populates endpoints stored in r into ClientConn (cc).
	r.updateState()
	return res, nil
}

func (r *RegattaManualResolver) SetEndpoints(endpoints []string) {
	r.endpoints = endpoints
	r.updateState()
}

func (r *RegattaManualResolver) updateState() {
	if r.CC != nil {
		addresses := make([]resolver.Address, len(r.endpoints))
		for i, ep := range r.endpoints {
			addr, serverName := endpoint.Interpret(ep)
			addresses[i] = resolver.Address{Addr: addr, ServerName: serverName}
		}
		state := resolver.State{
			Addresses:     addresses,
			ServiceConfig: r.serviceConfig,
		}
		r.UpdateState(state)
	}
}
