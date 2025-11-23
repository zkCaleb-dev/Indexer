package rpc

import (
	"context"
)

type LedgerHandler interface {
	PrepareRange(ctx context.Context, start, end *uint32) error
	HandleR()
}
