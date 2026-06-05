// Package iqplus_publisher wires the IQPlus TCP client to a JetStream
// publisher. It does not register HTTP routes — the module exposes only an
// Initialize() + Run() lifecycle for use from a dedicated cmd binary.
package iqplus_publisher

import (
	"context"

	"tuai/internal/modules/stock/iqplus_publisher/client"
	"tuai/internal/modules/stock/iqplus_publisher/publisher"
	"tuai/internal/modules/stock/iqplus_publisher/service"
)

// Module bundles the long-lived components.
type Module struct {
	Client    *client.Client
	Publisher *publisher.Publisher
	Service   *service.Service
}

// Config is the union of all sub-configs needed to spin the module up.
type Config struct {
	Client    client.Config
	Publisher publisher.Config
	Service   service.Config
}

// Initialize constructs the module without performing any I/O. Call Run()
// to actually connect to NATS and IQPlus.
func Initialize(cfg Config) *Module {
	c := client.New(cfg.Client)
	p := publisher.New(cfg.Publisher)
	s := service.New(c, p, cfg.Service)
	return &Module{
		Client:    c,
		Publisher: p,
		Service:   s,
	}
}

// Run blocks until ctx is cancelled. Returns ctx.Err() on graceful shutdown
// or the first fatal error from the JetStream connection.
func (m *Module) Run(ctx context.Context) error {
	return m.Service.Run(ctx)
}
