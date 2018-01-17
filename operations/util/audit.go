package util

import (
	"github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"golang.org/x/net/context"
)

func AddAuditData(ctx context.Context, key, value string) error {
	grpc_ctxtags.Extract(ctx).Set(key, value)
	return nil
}
