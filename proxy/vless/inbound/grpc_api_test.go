package inbound

import (
	"context"
	"strings"
	"testing"

	"github.com/xtls/xray-core/common/uuid"
	"github.com/xtls/xray-core/proxy/vless"
	"golang.org/x/time/rate"
)

func TestAddUser_WithRate(t *testing.T) {
	validator := &vless.MemoryValidator{}
	api := NewApi(validator)

	req := &AddUserRequest{
		Id:            "b0e5e4c0-6c6c-4c6c-8c6c-6c6c6c6c6c6c",
		TxBytesPerSec: 1000,
		TxBurstBytes:  2000,
		RxBytesPerSec: 500,
		RxBurstBytes:  0,
	}

	resp, err := api.AddUser(context.Background(), req)
	if err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("AddUser failed: %s", resp.Error)
	}

	id, _ := uuid.ParseString(req.Id)
	user := validator.Get(id)
	if user == nil {
		t.Fatal("User not found in validator")
	}

	acc := user.Account.(*vless.MemoryAccount)

	txLim := acc.TxLimiter.Load()
	if txLim == nil {
		t.Fatal("TxLimiter was not set")
	}
	if txLim.Limit() != rate.Limit(1000) {
		t.Errorf("TxLimiter Limit = %v, want %v", txLim.Limit(), rate.Limit(1000))
	}
	if txLim.Burst() != 2000 {
		t.Errorf("TxLimiter Burst = %d, want %d", txLim.Burst(), 2000)
	}

	rxLim := acc.RxLimiter.Load()
	if rxLim == nil {
		t.Fatal("RxLimiter was not set")
	}
	if rxLim.Limit() != rate.Limit(500) {
		t.Errorf("RxLimiter Limit = %v, want %v", rxLim.Limit(), rate.Limit(500))
	}
	if rxLim.Burst() != 1000 {
		t.Errorf("RxLimiter Burst = %d, want %d", rxLim.Burst(), 1000)
	}
}

func TestUpdateUserRate_Success(t *testing.T) {
	validator := &vless.MemoryValidator{}
	api := NewApi(validator)

	addReq := &AddUserRequest{
		Id:            "b0e5e4c0-6c6c-4c6c-8c6c-6c6c6c6c6c6c",
		TxBytesPerSec: 1000,
		TxBurstBytes:  2000,
		RxBytesPerSec: 500,
		RxBurstBytes:  1000,
	}
	addResp, err := api.AddUser(context.Background(), addReq)
	if err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if addResp.Error != "" {
		t.Fatalf("AddUser failed: %s", addResp.Error)
	}

	updateReq := &UpdateUserRateRequest{
		Id:            "b0e5e4c0-6c6c-4c6c-8c6c-6c6c6c6c6c6c",
		TxBytesPerSec: 2000,
		TxBurstBytes:  4000,
		RxBytesPerSec: 1500,
		RxBurstBytes:  3000,
	}
	updateResp, err := api.UpdateUserRate(context.Background(), updateReq)
	if err != nil {
		t.Fatalf("UpdateUserRate returned error: %v", err)
	}
	if updateResp.Error != "" {
		t.Fatalf("UpdateUserRate failed: %s", updateResp.Error)
	}

	id, _ := uuid.ParseString(addReq.Id)
	user := validator.Get(id)
	if user == nil {
		t.Fatal("User not found in validator")
	}
	acc := user.Account.(*vless.MemoryAccount)

	txLim := acc.TxLimiter.Load()
	if txLim == nil {
		t.Fatal("TxLimiter was not set after update")
	}
	if txLim.Limit() != rate.Limit(2000) {
		t.Errorf("Updated TxLimiter Limit = %v, want %v", txLim.Limit(), rate.Limit(2000))
	}
	if txLim.Burst() != 4000 {
		t.Errorf("Updated TxLimiter Burst = %d, want %d", txLim.Burst(), 4000)
	}

	rxLim := acc.RxLimiter.Load()
	if rxLim == nil {
		t.Fatal("RxLimiter was not set after update")
	}
	if rxLim.Limit() != rate.Limit(1500) {
		t.Errorf("Updated RxLimiter Limit = %v, want %v", rxLim.Limit(), rate.Limit(1500))
	}
	if rxLim.Burst() != 3000 {
		t.Errorf("Updated RxLimiter Burst = %d, want %d", rxLim.Burst(), 3000)
	}
}

func TestUpdateUserRate_NotFound(t *testing.T) {
	validator := &vless.MemoryValidator{}
	api := NewApi(validator)

	updateReq := &UpdateUserRateRequest{
		Id:            "99999999-9999-9999-9999-999999999999",
		TxBytesPerSec: 2000,
		TxBurstBytes:  4000,
		RxBytesPerSec: 1500,
		RxBurstBytes:  3000,
	}
	updateResp, err := api.UpdateUserRate(context.Background(), updateReq)
	if err != nil {
		t.Fatalf("UpdateUserRate returned unexpected error: %v", err)
	}
	if updateResp.Error == "" {
		t.Fatal("Expected error for non-existent user, got none")
	}
	if !strings.Contains(updateResp.Error, "not found") {
		t.Errorf("Error message = %q, want to contain %q", updateResp.Error, "not found")
	}
}
