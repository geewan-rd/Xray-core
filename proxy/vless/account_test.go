package vless

import (
	"testing"

	"golang.org/x/time/rate"
)

func TestMemoryAccount_SetRate(t *testing.T) {
	acc := &MemoryAccount{}

	if acc.TxLimiter.Load() != nil {
		t.Fatal("Initial TxLimiter should be nil")
	}
	if acc.RxLimiter.Load() != nil {
		t.Fatal("Initial RxLimiter should be nil")
	}

	acc.SetRate(1024, 0, 0, 0)
	txLim := acc.TxLimiter.Load()
	if txLim == nil {
		t.Fatal("TxLimiter should be non-nil after SetRate(1024, 0, 0, 0)")
	}
	if txLim.Limit() != rate.Limit(1024) {
		t.Errorf("TxLimiter Limit() = %v, want 1024", txLim.Limit())
	}
	if txLim.Burst() != 2048 {
		t.Errorf("TxLimiter Burst() = %d, want 2048 (default 2*bps)", txLim.Burst())
	}
	if acc.RxLimiter.Load() != nil {
		t.Error("RxLimiter should remain nil")
	}

	acc.SetRate(0, 0, 2048, 8192)
	if acc.TxLimiter.Load() != nil {
		t.Error("TxLimiter should be nil after SetRate(0, ...)")
	}
	rxLim := acc.RxLimiter.Load()
	if rxLim == nil {
		t.Fatal("RxLimiter should be non-nil after SetRate(0, 0, 2048, 8192)")
	}
	if rxLim.Limit() != rate.Limit(2048) {
		t.Errorf("RxLimiter Limit() = %v, want 2048", rxLim.Limit())
	}
	if rxLim.Burst() != 8192 {
		t.Errorf("RxLimiter Burst() = %d, want 8192", rxLim.Burst())
	}

	acc.SetRate(0, 0, 0, 0)
	if acc.TxLimiter.Load() != nil {
		t.Error("TxLimiter should be nil after SetRate(0, 0, 0, 0)")
	}
	if acc.RxLimiter.Load() != nil {
		t.Error("RxLimiter should be nil after SetRate(0, 0, 0, 0)")
	}

	acc.SetRate(500, 1000, 600, 0)
	txLim = acc.TxLimiter.Load()
	rxLim = acc.RxLimiter.Load()
	if txLim == nil || rxLim == nil {
		t.Fatal("Both limiters should be non-nil after SetRate(500, 1000, 600, 0)")
	}
	if txLim.Limit() != rate.Limit(500) {
		t.Errorf("TxLimiter Limit() = %v, want 500", txLim.Limit())
	}
	if txLim.Burst() != 1000 {
		t.Errorf("TxLimiter Burst() = %d, want 1000 (explicit)", txLim.Burst())
	}
	if rxLim.Limit() != rate.Limit(600) {
		t.Errorf("RxLimiter Limit() = %v, want 600", rxLim.Limit())
	}
	if rxLim.Burst() != 1200 {
		t.Errorf("RxLimiter Burst() = %d, want 1200 (default 2*600)", rxLim.Burst())
	}
}
