package orm

import (
	"context"
	"fmt"
)

func (d *Database) Ready(ctx context.Context) error {
	pool, err := d.SQL()
	if err != nil {
		return fmt.Errorf("database is not ready")
	}
	if err = pool.PingContext(ctx); err != nil {
		return fmt.Errorf("database is not ready")
	}
	return nil
}
