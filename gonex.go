// Package gonex constructs applications with explicit modules.
package gonex

import (
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
)

func New(c config.Config) (*app.App, error) { return app.New(c) }
