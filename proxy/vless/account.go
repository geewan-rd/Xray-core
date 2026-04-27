package vless

import (
	"sync/atomic"

	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/uuid"
)

// AsAccount implements protocol.Account.AsAccount().
func (a *Account) AsAccount() (protocol.Account, error) {
	id, err := uuid.ParseString(a.Id)
	if err != nil {
		return nil, errors.New("failed to parse ID").Base(err).AtError()
	}
	return &MemoryAccount{
		ID:         protocol.NewID(id),
		Flow:       a.Flow,       // needs parser here?
		Encryption: a.Encryption, // needs parser here?
	}, nil
}

// MemoryAccount is an in-memory form of VLess account.
type MemoryAccount struct {
	// ID of the account.
	ID *protocol.ID
	// Flow of the account. May be "xtls-rprx-vision".
	Flow string
	// Encryption of the account. Used for client connections, and only accepts "none" for now.
	Encryption string
	// TxLimiter limits server→client traffic (client downlink).
	TxLimiter atomic.Pointer[rate.Limiter]
	// RxLimiter limits client→server traffic (client uplink).
	RxLimiter atomic.Pointer[rate.Limiter]
}

// Equals implements protocol.Account.Equals().
func (a *MemoryAccount) Equals(account protocol.Account) bool {
	vlessAccount, ok := account.(*MemoryAccount)
	if !ok {
		return false
	}
	return a.ID.Equals(vlessAccount.ID)
}

func (a *MemoryAccount) ToProto() proto.Message {
	return &Account{
		Id:         a.ID.String(),
		Flow:       a.Flow,
		Encryption: a.Encryption,
	}
}

// SetRate configures rate limiters for both directions.
// txBps/rxBps: bytes per second (0 = unlimited).
// txBurst/rxBurst: burst size in bytes (0 = default to 2*bps).
func (a *MemoryAccount) SetRate(txBps, txBurst, rxBps, rxBurst uint64) {
	if txBps == 0 {
		a.TxLimiter.Store(nil)
	} else {
		burst := int(txBurst)
		if burst == 0 {
			burst = int(txBps) * 2
		}
		a.TxLimiter.Store(rate.NewLimiter(rate.Limit(txBps), burst))
	}

	if rxBps == 0 {
		a.RxLimiter.Store(nil)
	} else {
		burst := int(rxBurst)
		if burst == 0 {
			burst = int(rxBps) * 2
		}
		a.RxLimiter.Store(rate.NewLimiter(rate.Limit(rxBps), burst))
	}
}
