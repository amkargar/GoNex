package app

import (
	"context"
	"fmt"
	apperrors "github.com/amkargar/gonex/errors"
	"time"
)

type readinessCheck struct {
	name  string
	check func(context.Context) error
}

func (r *Registrar) Check(name string, check func(context.Context) error) error {
	if r.sealed || name == "" || check == nil {
		return fmt.Errorf("invalid or closed readiness registration")
	}
	for _, existing := range r.checks {
		if existing.name == name {
			return fmt.Errorf("duplicate readiness check %s", name)
		}
	}
	r.checks = append(r.checks, readinessCheck{name, check})
	return nil
}
func (a *App) Ready(ctx context.Context) error {
	a.mu.Lock()
	running := a.state == Running
	notReady := func() error { return apperrors.New(apperrors.Unavailable, "service not ready") }
	if !running {
		a.mu.Unlock()
		return notReady()
	}
	checks := append([]readinessCheck(nil), a.registrar.checks...)
	resources := append([]Resource(nil), a.started...)
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	for _, resource := range resources {
		if ready, ok := resource.(interface{ Ready(context.Context) error }); ok {
			if ctx.Err() != nil || ready.Ready(ctx) != nil {
				return notReady()
			}
		}
	}
	for _, check := range checks {
		if ctx.Err() != nil || check.check(ctx) != nil {
			return notReady()
		}
	}
	if ctx.Err() != nil {
		return notReady()
	}
	return nil
}
