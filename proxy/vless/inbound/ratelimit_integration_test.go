package inbound_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/proxy/vless"
)

// mockReader implements buf.Reader for testing
type mockReader struct {
	data   []byte
	offset int
}

func (m *mockReader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	if m.offset >= len(m.data) {
		return nil, io.EOF
	}

	chunkSize := 8192 // 8KB per read (matches buf.Size)
	remaining := len(m.data) - m.offset
	if remaining > chunkSize {
		remaining = chunkSize
	}

	b := buf.New()
	_, _ = b.Write(m.data[m.offset : m.offset+remaining])
	m.offset += remaining

	return buf.MultiBuffer{b}, nil
}

// mockWriter implements buf.Writer for testing
type mockWriter struct {
	written int
	mu      sync.Mutex
}

func (m *mockWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	m.mu.Lock()
	m.written += int(mb.Len())
	m.mu.Unlock()
	buf.ReleaseMulti(mb)
	return nil
}

// TestRatelimitInProcess_1MBps_BothDirections tests 1MB/s rate limit in both directions
// Acceptance: C1 - 1MB/s rate should measure within [900, 1100] KB/s (±10%)
func TestRatelimitInProcess_1MBps_BothDirections(t *testing.T) {
	ctx := context.Background()
	account := &vless.MemoryAccount{}

	const rateBps = 1024 * 1024
	const burstBytes = 100 * 1024
	account.SetRate(rateBps, burstBytes, rateBps, burstBytes)

	dataSize := 4 * 1024 * 1024
	data := make([]byte, dataSize)

	srcReader := &mockReader{data: data}
	wrappedReader := buf.NewRateLimitReader(ctx, srcReader, &account.RxLimiter)

	start := time.Now()
	totalRead := 0
	for {
		mb, err := wrappedReader.ReadMultiBuffer()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("uplink read error: %v", err)
		}
		totalRead += int(mb.Len())
		buf.ReleaseMulti(mb)
	}
	elapsed := time.Since(start)

	if totalRead != dataSize {
		t.Errorf("uplink: expected to read %d bytes, got %d", dataSize, totalRead)
	}

	kbps := float64(totalRead) / elapsed.Seconds() / 1024
	if kbps < 900 || kbps > 1100 {
		t.Errorf("uplink: expected ~1024 KB/s (±10%%), got %.1f KB/s (elapsed: %v)", kbps, elapsed)
	}

	// Test downlink (server→client, use TxLimiter)
	dstWriter := &mockWriter{}
	wrappedWriter := buf.NewRateLimitWriter(ctx, dstWriter, &account.TxLimiter)

	mb := buf.MergeBytes(nil, data)
	start = time.Now()
	if err := wrappedWriter.WriteMultiBuffer(mb); err != nil {
		t.Fatalf("downlink write error: %v", err)
	}
	elapsed = time.Since(start)

	if dstWriter.written != dataSize {
		t.Errorf("downlink: expected to write %d bytes, got %d", dataSize, dstWriter.written)
	}

	kbps = float64(dstWriter.written) / elapsed.Seconds() / 1024
	if kbps < 900 || kbps > 1100 {
		t.Errorf("downlink: expected ~1024 KB/s (±10%%), got %.1f KB/s (elapsed: %v)", kbps, elapsed)
	}
}

// TestRatelimitInProcess_NoLimit tests unlimited rate (SetRate 0)
// Acceptance: C1 - 0 rate should not limit (>5000 KB/s equivalent, i.e., instant completion)
func TestRatelimitInProcess_NoLimit(t *testing.T) {
	ctx := context.Background()
	account := &vless.MemoryAccount{}

	// Set unlimited rate
	account.SetRate(0, 0, 0, 0)

	dataSize := 50 * 1024 * 1024 // 50MB
	data := make([]byte, dataSize)

	srcReader := &mockReader{data: data}
	wrappedReader := buf.NewRateLimitReader(ctx, srcReader, &account.RxLimiter)

	start := time.Now()
	totalRead := 0
	for {
		mb, err := wrappedReader.ReadMultiBuffer()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read error: %v", err)
		}
		totalRead += int(mb.Len())
		buf.ReleaseMulti(mb)
	}
	elapsed := time.Since(start)

	if totalRead != dataSize {
		t.Errorf("expected to read %d bytes, got %d", dataSize, totalRead)
	}

	if elapsed > 500*time.Millisecond {
		t.Errorf("unlimited rate took too long: %v (expected < 500ms)", elapsed)
	}
}

// TestRatelimitInProcess_TinyRate tests very low rate (1KB/s)
// Acceptance: C1 - 1KB/s rate should measure < 1.5 KB/s (accounting for burst)
func TestRatelimitInProcess_TinyRate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	account := &vless.MemoryAccount{}

	const rateBps = 1024
	const burstBytes = 512
	account.SetRate(rateBps, burstBytes, 0, 0)

	dataSize := 10 * 1024
	data := make([]byte, dataSize)

	srcReader := &mockReader{data: data}
	wrappedReader := buf.NewRateLimitReader(ctx, srcReader, &account.TxLimiter)

	start := time.Now()
	totalRead := 0
	for {
		mb, err := wrappedReader.ReadMultiBuffer()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("timeout after reading %d bytes (expected %d)", totalRead, dataSize)
			}
			t.Fatalf("read error: %v", err)
		}
		totalRead += int(mb.Len())
		buf.ReleaseMulti(mb)
	}
	elapsed := time.Since(start)

	if totalRead != dataSize {
		t.Errorf("expected to read %d bytes, got %d", dataSize, totalRead)
	}

	bps := float64(totalRead) / elapsed.Seconds()
	if bps >= 1500 {
		t.Errorf("expected < 1500 B/s, got %.1f B/s (elapsed: %v)", bps, elapsed)
	}
}

// TestRatelimitInProcess_LargeRate tests high rate (10MB/s)
// Acceptance: C1 - large rate should achieve near-physical limit (> 8MB/s)
func TestRatelimitInProcess_LargeRate(t *testing.T) {
	ctx := context.Background()
	account := &vless.MemoryAccount{}

	const rateBps = 10 * 1024 * 1024
	const burstBytes = 1 * 1024 * 1024
	account.SetRate(0, 0, rateBps, burstBytes)

	dataSize := 20 * 1024 * 1024
	data := make([]byte, dataSize)

	dstWriter := &mockWriter{}
	wrappedWriter := buf.NewRateLimitWriter(ctx, dstWriter, &account.RxLimiter)

	mb := buf.MergeBytes(nil, data)
	start := time.Now()
	if err := wrappedWriter.WriteMultiBuffer(mb); err != nil {
		t.Fatalf("write error: %v", err)
	}
	elapsed := time.Since(start)

	if dstWriter.written != dataSize {
		t.Errorf("expected to write %d bytes, got %d", dataSize, dstWriter.written)
	}

	kbps := float64(dstWriter.written) / elapsed.Seconds() / 1024
	if kbps < 8*1024 {
		t.Errorf("expected > 8 MB/s, got %.1f KB/s (elapsed: %v)", kbps, elapsed)
	}
}

// TestRatelimitInProcess_RateUpdateImmediate tests that SetRate immediately affects existing wrappers
// Acceptance: C2 equivalent - SetRate changes take immediate effect on next Read/Write
func TestRatelimitInProcess_RateUpdateImmediate(t *testing.T) {
	ctx := context.Background()
	account := &vless.MemoryAccount{}

	account.SetRate(1024*1024, 100*1024, 0, 0)

	dataSize := 8 * 1024 * 1024
	data := make([]byte, dataSize)

	srcReader := &mockReader{data: data}
	wrappedReader := buf.NewRateLimitReader(ctx, srcReader, &account.TxLimiter)

	// Read 2MB at initial rate (1MB/s) - should take ~2s
	chunk1Start := time.Now()
	chunk1Size := 0
	for chunk1Size < 2*1024*1024 {
		mb, err := wrappedReader.ReadMultiBuffer()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("chunk1 read error: %v", err)
		}
		chunk1Size += int(mb.Len())
		buf.ReleaseMulti(mb)
	}
	chunk1Elapsed := time.Since(chunk1Start)

	account.SetRate(4*1024*1024, 100*1024, 0, 0)

	chunk2Start := time.Now()
	chunk2Size := 0
	for {
		mb, err := wrappedReader.ReadMultiBuffer()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("chunk2 read error: %v", err)
		}
		chunk2Size += int(mb.Len())
		buf.ReleaseMulti(mb)
	}
	chunk2Elapsed := time.Since(chunk2Start)

	// Verify chunk1 rate ~1MB/s (±20% accounting for burst)
	chunk1Kbps := float64(chunk1Size) / chunk1Elapsed.Seconds() / 1024
	if chunk1Kbps < 800 || chunk1Kbps > 1300 {
		t.Errorf("chunk1: expected ~1024 KB/s, got %.1f KB/s", chunk1Kbps)
	}

	// Verify chunk2 rate ~4MB/s (±20% accounting for burst)
	chunk2Kbps := float64(chunk2Size) / chunk2Elapsed.Seconds() / 1024
	if chunk2Kbps < 3200 || chunk2Kbps > 5000 {
		t.Errorf("chunk2: expected ~4096 KB/s, got %.1f KB/s", chunk2Kbps)
	}

	t.Logf("chunk1: %.1f KB/s (%v), chunk2: %.1f KB/s (%v) - immediate rate update confirmed",
		chunk1Kbps, chunk1Elapsed, chunk2Kbps, chunk2Elapsed)
}

// TestRatelimitInProcess_CtxCancel tests context cancellation during rate-limited wait
// Acceptance: C3 - ctx cancel should return context.Canceled and not leak goroutine
func TestRatelimitInProcess_CtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	account := &vless.MemoryAccount{}

	// Set very slow rate (1 byte/s) to force blocking
	account.SetRate(1, 2, 0, 0)

	dataSize := 10 * 1024 // 10KB
	data := make([]byte, dataSize)

	srcReader := &mockReader{data: data}
	wrappedReader := buf.NewRateLimitReader(ctx, srcReader, &account.TxLimiter)

	// Cancel context after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	gotCanceled := false
	for {
		mb, err := wrappedReader.ReadMultiBuffer()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				gotCanceled = true
				break
			}
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
		buf.ReleaseMulti(mb)
	}
	elapsed := time.Since(start)

	if !gotCanceled {
		t.Errorf("expected context.Canceled error")
	}

	// Should return quickly after cancel (within 200ms)
	if elapsed > 200*time.Millisecond {
		t.Errorf("cancellation took too long: %v", elapsed)
	}
}
