package model

import (
	"context"

	"github.com/horockey/dkv"
	serdisc "github.com/horockey/service_discovery/api"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

var _ dkv.Discovery = &DiscoveryImpl{}

type DiscoveryImpl struct {
	Cl     *serdisc.Client
	Logger zerolog.Logger
}

func (impl *DiscoveryImpl) Register(ctx context.Context, hostname string, updCb func(dkv.Node) error, meta map[string]string) error {
	cb := func(n serdisc.Node) error {
		impl.Logger.
			Debug().
			Str("hostname", n.Hostname).
			Str("state", n.State).
			Msg("nodes upd")
		return updCb(dkv.Node(n))
	}
	return impl.Cl.Register(ctx, hostname, cb, meta)
}

func (impl *DiscoveryImpl) Deregister(ctx context.Context) error {
	return impl.Cl.Deregister(ctx)
}

func (impl *DiscoveryImpl) GetNodes(ctx context.Context) ([]dkv.Node, error) {
	nodes, err := impl.Cl.GetNodes(ctx)
	if err != nil {
		return nil, err
	}
	return lo.Map[serdisc.Node, dkv.Node](
			nodes,
			func(el serdisc.Node, _ int) dkv.Node {
				return dkv.Node(el)
			},
		),
		nil
}
