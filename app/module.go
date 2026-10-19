package app

import "context"

type Module interface{ Register(*Registrar) error }

// Resource must release partially acquired resources when Close follows a failed Start.
type Resource interface {
	Start(context.Context) error
	Close(context.Context) error
}
