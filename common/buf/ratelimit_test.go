package buf_test

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/xtls/xray-core/common/buf"
	"golang.org/x/time/rate"
)

// mockReader implements Reader for testing
type mockReader struct {
	data   []byte
	offset int
}

func (m *mockReader) ReadMultiBuffer() (MultiBuffer, error) {
	if m.offset >= len(m.data) {
		return nil, io.EOF
	}

	chunkSize := 8192 // 8KB per read
	remaining := len(m.data) - m.offset
	if remaining > chunkSize {
		remaining = chunkSize
	}

	b := New()
	_, _ = b.Write(m.data[m.offset : m.offset+remaining])
	m.offset += remaining

	return MultiBuffer{b}, nil
}

// mockWriter implements Writer for testing
type mockWriter struct {
	written int
}

func (m *mockWriter) WriteMultiBuffer(mb MultiBuffer) error {
	m.written += int(mb.Len())
	ReleaseMulti(mb)
	return nil
}

// TestRateLimit_PassThrough_NilLimiter tests pass-through when limiter is nil
func TestRateLimit_PassThrough_NilLimiter(t *testing.T) {
	ctx := context.Background()
	data := make([]byte, 1024*1024) // 1MB

	// Test Reader
	var limPtr atomic.Pointer[rate.Limiter]
	reader := &mockReader{data: data}
	rlReader := NewRateLimitReader(ctx, reader, &limPtr)

	start := time.Now()
	totalRead := 0
	for {
		mb, err := rlReader.ReadMultiBuffer()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		totalRead += int(mb.Len())
		ReleaseMulti(mb)
	}
	elapsed := time.Since(start)

	if totalRead != len(data) {
		t.Errorf("expected to read %d bytes, got %d", len(data), totalRead)
	}

	// Should complete very fast without rate limiting (< 100ms)
	if elapsed > 100*time.Millisecond {
		t.Errorf("pass-through took too long: %v", elapsed)
	}

	// Test Writer
	writer := &mockWriter{}
	rlWriter := NewRateLimitWriter(ctx, writer, &limPtr)

	mb := MergeBytes(nil, data)

	start = time.Now()
	if err := rlWriter.WriteMultiBuffer(mb); err != nil {
		t.Fatalf("write error: %v", err)
	}
	elapsed = time.Since(start)

	if writer.written != len(data) {
		t.Errorf("expected to write %d bytes, got %d", len(data), writer.written)
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("pass-through write took too long: %v", elapsed)
	}
}

// TestRateLimitReader_Throughput tests 1MB/s rate with 5MB data
func TestRateLimitReader_Throughput(t *testing.T) {
	ctx := context.Background()
	data := make([]byte, 5*1024*1024) // 5MB

	// 1MB/s with 100KB burst
	lim := rate.NewLimiter(rate.Limit(1024*1024), 100*1024)
	var limPtr atomic.Pointer[rate.Limiter]
	limPtr.Store(lim)

	reader := &mockReader{data: data}
	rlReader := NewRateLimitReader(ctx, reader, &limPtr)

	start := time.Now()
	totalRead := 0
	for {
		mb, err := rlReader.ReadMultiBuffer()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		totalRead += int(mb.Len())
		ReleaseMulti(mb)
	}
	elapsed := time.Since(start)

	if totalRead != len(data) {
		t.Errorf("expected to read %d bytes, got %d", len(data), totalRead)
	}

	// Should take ~5s ± 10% (4.5s to 5.5s)
	expectedDuration := 5 * time.Second
	minDuration := 4500 * time.Millisecond
	maxDuration := 5500 * time.Millisecond

	if elapsed < minDuration || elapsed > maxDuration {
		t.Errorf("expected duration ~%v (±10%%), got %v", expectedDuration, elapsed)
	}
}

// TestRateLimitWriter_LargeBuffer tests burst=10KB with 100KB buffer
func TestRateLimitWriter_LargeBuffer(t *testing.T) {
	ctx := context.Background()
	data := make([]byte, 100*1024) // 100KB

	// 1MB/s with 10KB burst - single buffer > burst
	lim := rate.NewLimiter(rate.Limit(1024*1024), 10*1024)
	var limPtr atomic.Pointer[rate.Limiter]
	limPtr.Store(lim)

	writer := &mockWriter{}
	rlWriter := NewRateLimitWriter(ctx, writer, &limPtr)

	mb := MergeBytes(nil, data)

	start := time.Now()
	if err := rlWriter.WriteMultiBuffer(mb); err != nil {
		t.Fatalf("write error: %v", err)
	}
	elapsed := time.Since(start)

	if writer.written != len(data) {
		t.Errorf("expected to write %d bytes, got %d", len(data), writer.written)
	}

	// Should take ~100ms ± 20% (80ms to 120ms)
	expectedDuration := 100 * time.Millisecond
	minDuration := 80 * time.Millisecond
	maxDuration := 120 * time.Millisecond

	if elapsed < minDuration || elapsed > maxDuration {
		t.Errorf("expected duration ~%v (±20%%), got %v", expectedDuration, elapsed)
	}
}

// TestRateLimit_CtxCancel tests context cancellation
func TestRateLimit_CtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	data := make([]byte, 10*1024*1024) // 10MB

	// Very slow rate: 100KB/s with 10KB burst
	lim := rate.NewLimiter(rate.Limit(100*1024), 10*1024)
	var limPtr atomic.Pointer[rate.Limiter]
	limPtr.Store(lim)

	reader := &mockReader{data: data}
	rlReader := NewRateLimitReader(ctx, reader, &limPtr)

	// Cancel context after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	for {
		mb, err := rlReader.ReadMultiBuffer()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				break
			}
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
		ReleaseMulti(mb)
	}
	elapsed := time.Since(start)

	// Should cancel quickly (within 200ms)
	if elapsed > 200*time.Millisecond {
		t.Errorf("cancellation took too long: %v", elapsed)
	}
}

// TestRateLimit_AtomicSwap tests swapping limiter mid-flight
func TestRateLimit_AtomicSwap(t *testing.T) {
	ctx := context.Background()
	data := make([]byte, 2*1024*1024) // 2MB

	// Start with 1MB/s
	lim1 := rate.NewLimiter(rate.Limit(1024*1024), 1024*1024)
	var limPtr atomic.Pointer[rate.Limiter]
	limPtr.Store(lim1)

	reader := &mockReader{data: data}
	rlReader := NewRateLimitReader(ctx, reader, &limPtr)

	readCount := 0
	start := time.Now()

	for {
		// After first read, swap to faster limiter (10MB/s)
		if readCount == 1 {
			lim2 := rate.NewLimiter(rate.Limit(10*1024*1024), 10*1024*1024)
			limPtr.Store(lim2)
		}

		mb, err := rlReader.ReadMultiBuffer()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		readCount++
		ReleaseMulti(mb)
	}
	elapsed := time.Since(start)

	// First chunk slow, rest fast - should be faster than 2s
	if elapsed > 2*time.Second {
		t.Errorf("swap didn't speed up reads: %v", elapsed)
	}
}

// TestRateLimitReader_BasicSmall tests small buffer < burst
func TestRateLimitReader_BasicSmall(t *testing.T) {
	ctx := context.Background()
	data := make([]byte, 1024) // 1KB

	// 1MB/s with 10KB burst
	lim := rate.NewLimiter(rate.Limit(1024*1024), 10*1024)
	var limPtr atomic.Pointer[rate.Limiter]
	limPtr.Store(lim)

	reader := &mockReader{data: data}
	rlReader := NewRateLimitReader(ctx, reader, &limPtr)

	start := time.Now()
	mb, err := rlReader.ReadMultiBuffer()
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)

	if mb.Len() != int32(len(data)) {
		t.Errorf("expected to read %d bytes, got %d", len(data), mb.Len())
	}
	ReleaseMulti(mb)

	// Small read should complete very fast (< 10ms)
	if elapsed > 10*time.Millisecond {
		t.Errorf("small read took too long: %v", elapsed)
	}
}
