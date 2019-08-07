package kube

import (
	"context"
	"io"
)

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func newCtxReader(ctx context.Context, r io.Reader) *ctxReader {
	return &ctxReader{ctx, r}
}

func (r *ctxReader) Read(buf []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.r.Read(buf)
	}
}
