package stats_test

import (
	"sync"
	"testing"

	. "github.com/xtls/xray-core/app/stats"
)

func TestOnlineMapConcurrentAddIP(t *testing.T) {
	om := NewOnlineMap()

	// Simulate high-concurrency scenario: many goroutines calling AddIP simultaneously.
	// Without proper locking in AddIP, this triggers:
	//   fatal error: concurrent map read and map write
	//   fatal error: concurrent map writes
	const goroutines = 100
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Use varied IPs to exercise both the "key exists" and "key new" paths
				// Some goroutines will use the same IP to trigger the read-before-write race
				switch j % 4 {
				case 0:
					om.AddIP("10.0.0.1") // shared IP — triggers read race on existing key
				case 1:
					om.AddIP("192.168.0." + string(rune('0'+id%10))) // semi-shared
				case 2:
					om.AddIP("172.16.0." + string(rune('0'+id))) // unique-ish
				case 3:
					om.AddIP("127.0.0.1") // should be skipped (localhost check)
				}
			}
		}(i)
	}

	wg.Wait()

	// After all goroutines finish, verify the map is in a consistent state
	count := om.Count()
	if count < 0 {
		t.Fatalf("unexpected negative count: %d", count)
	}
	list := om.List()
	if len(list) != count {
		t.Fatalf("List() length %d != Count() %d", len(list), count)
	}
}

func TestOnlineMapConcurrentAddIPAndGetKeys(t *testing.T) {
	om := NewOnlineMap()

	// AddIP and GetKeys/List must be safe to call concurrently.
	// GetKeys holds RLock but AddIP does NOT hold any lock for the initial map read,
	// so concurrent AddIP + GetKeys triggers: concurrent map read and map write
	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2) // half writers, half readers

	// Writers: call AddIP
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				om.AddIP("10.0." + string(rune('0'+id%10)) + "." + string(rune('0'+j%10)))
			}
		}(i)
	}

	// Readers: call GetKeys and List
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = om.List()
				_ = om.Count()
			}
		}()
	}

	wg.Wait()
}

func TestOnlineMapAddIPSkipsLocalhost(t *testing.T) {
	om := NewOnlineMap()

	om.AddIP("127.0.0.1")
	if om.Count() != 0 {
		t.Fatalf("localhost should be skipped, got count %d", om.Count())
	}
	if len(om.List()) != 0 {
		t.Fatalf("localhost should be skipped, got list %v", om.List())
	}

	om.AddIP("10.0.0.1")
	if om.Count() != 1 {
		t.Fatalf("expected count 1 after adding non-localhost, got %d", om.Count())
	}
}
