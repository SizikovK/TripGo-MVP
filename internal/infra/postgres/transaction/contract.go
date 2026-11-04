package transaction

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type database interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}
