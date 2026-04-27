package buf

import (
	"context"
	"sync/atomic"

	"golang.org/x/time/rate"
)

// RateLimitReader wraps a Reader with rate limiting support.
type RateLimitReader struct {
	ctx    context.Context
	reader Reader
	limPtr *atomic.Pointer[rate.Limiter]
}

// NewRateLimitReader creates a new rate-limited Reader.
// If limPtr.Load() returns nil, the reader operates in pass-through mode.
func NewRateLimitReader(ctx context.Context, r Reader, limPtr *atomic.Pointer[rate.Limiter]) Reader {
	return &RateLimitReader{
		ctx:    ctx,
		reader: r,
		limPtr: limPtr,
	}
}

// ReadMultiBuffer implements Reader interface with rate limiting.
func (r *RateLimitReader) ReadMultiBuffer() (MultiBuffer, error) {
	mb, err := r.reader.ReadMultiBuffer()
	if err != nil {
		return mb, err
	}

	lim := r.limPtr.Load()
	if lim == nil {
		return mb, nil
	}

	n := int(mb.Len())
	if n == 0 {
		return mb, nil
	}

	burst := lim.Burst()
	for remaining := n; remaining > 0; {
		chunk := remaining
		if chunk > burst {
			chunk = burst
		}
		if err := lim.WaitN(r.ctx, chunk); err != nil {
			ReleaseMulti(mb)
			return nil, err
		}
		remaining -= chunk
	}

	return mb, nil
}

// RateLimitWriter wraps a Writer with rate limiting support.
type RateLimitWriter struct {
	ctx    context.Context
	writer Writer
	limPtr *atomic.Pointer[rate.Limiter]
}

// NewRateLimitWriter creates a new rate-limited Writer.
// If limPtr.Load() returns nil, the writer operates in pass-through mode.
func NewRateLimitWriter(ctx context.Context, w Writer, limPtr *atomic.Pointer[rate.Limiter]) Writer {
	return &RateLimitWriter{
		ctx:    ctx,
		writer: w,
		limPtr: limPtr,
	}
}

// WriteMultiBuffer implements Writer interface with rate limiting.
func (w *RateLimitWriter) WriteMultiBuffer(mb MultiBuffer) error {
	lim := w.limPtr.Load()
	if lim == nil {
		return w.writer.WriteMultiBuffer(mb)
	}

	n := int(mb.Len())
	if n == 0 {
		return w.writer.WriteMultiBuffer(mb)
	}

	burst := lim.Burst()
	for remaining := n; remaining > 0; {
		chunk := remaining
		if chunk > burst {
			chunk = burst
		}
		if err := lim.WaitN(w.ctx, chunk); err != nil {
			return err
		}
		remaining -= chunk
	}

	return w.writer.WriteMultiBuffer(mb)
}
