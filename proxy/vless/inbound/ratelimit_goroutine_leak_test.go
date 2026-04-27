package inbound_test

import (
	"context"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/proxy/vless"
)

// TestRatelimit_NoGoroutineLeak tests that rate-limited wrappers don't leak goroutines
// Acceptance: C3 - 100 concurrent wrappers should not leak (NumGoroutine delta < 10)
func TestRatelimit_NoGoroutineLeak(t *testing.T) {
	// Force GC to establish clean baseline
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	baselineGoroutines := runtime.NumGoroutine()

	// Create 100 concurrent rate-limited readers/writers
	const numWorkers = 100
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(id int) {
			defer wg.Done()

			ctx := context.Background()
			account := &vless.MemoryAccount{}

			account.SetRate(1024*1024, 100*1024, 1024*1024, 100*1024)
			dataSize := 100 * 1024
			data := make([]byte, dataSize)

			srcReader := &mockReader{data: data}
			wrappedReader := buf.NewRateLimitReader(ctx, srcReader, &account.TxLimiter)

			totalRead := 0
			for {
				mb, err := wrappedReader.ReadMultiBuffer()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Errorf("worker %d: read error: %v", id, err)
					return
				}
				totalRead += int(mb.Len())
				buf.ReleaseMulti(mb)
			}

			// Push 100KB through wrapped writer
			dstWriter := &mockWriter{}
			wrappedWriter := buf.NewRateLimitWriter(ctx, dstWriter, &account.RxLimiter)

			mb := buf.MergeBytes(nil, data)
			if err := wrappedWriter.WriteMultiBuffer(mb); err != nil {
				t.Errorf("worker %d: write error: %v", id, err)
			}
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()

	// Force GC to clean up any lingering resources
	runtime.GC()
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	delta := finalGoroutines - baselineGoroutines

	if delta > 10 {
		t.Errorf("goroutine leak detected: baseline=%d, final=%d, delta=%d (expected < 10)",
			baselineGoroutines, finalGoroutines, delta)
	}

	t.Logf("goroutine count: baseline=%d, final=%d, delta=%d", baselineGoroutines, finalGoroutines, delta)
}
