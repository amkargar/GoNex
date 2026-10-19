package main

import (
	"context"
	"github.com/amkargar/gonex/app"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/observability"
)

type telemetryModule struct {
	Metrics *observability.Metrics
	Expose  bool
}

func (m telemetryModule) Register(r *app.Registrar) error {
	if err := r.Manage(m.Metrics); err != nil {
		return err
	}
	if !m.Expose {
		return nil
	}
	return transport.Register(r, transport.Endpoint[transport.NoBody, observability.Snapshot]{Method: "GET", Path: "/metrics", OperationID: "metrics", Status: 200, Handler: func(ctx context.Context, _ transport.NoBody) (observability.Snapshot, error) {
		return m.Metrics.SnapshotContext(ctx), nil
	}})
}
