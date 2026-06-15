package mux_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/mux"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/pipe"
)

func newLinkPair() (*transport.Link, *transport.Link) {
	opt := pipe.WithoutSizeLimit()
	uplinkReader, uplinkWriter := pipe.New(opt)
	downlinkReader, downlinkWriter := pipe.New(opt)

	uplink := &transport.Link{
		Reader: uplinkReader,
		Writer: downlinkWriter,
	}

	downlink := &transport.Link{
		Reader: downlinkReader,
		Writer: uplinkWriter,
	}

	return uplink, downlink
}

type TestDispatcher struct {
	OnDispatch func(ctx context.Context, dest net.Destination) (*transport.Link, error)
}

func (d *TestDispatcher) Dispatch(ctx context.Context, dest net.Destination) (*transport.Link, error) {
	return d.OnDispatch(ctx, dest)
}

func (d *TestDispatcher) DispatchLink(ctx context.Context, destination net.Destination, outbound *transport.Link) error {
	return nil
}

func (d *TestDispatcher) Start() error {
	return nil
}

func (d *TestDispatcher) Close() error {
	return nil
}

func (*TestDispatcher) Type() interface{} {
	return routing.DispatcherType()
}

func TestRegressionOutboundLeak(t *testing.T) {
	originalOutbounds := []*session.Outbound{{}}
	serverCtx := session.ContextWithOutbounds(context.Background(), originalOutbounds)

	websiteUplink, websiteDownlink := newLinkPair()

	dispatcher := TestDispatcher{
		OnDispatch: func(ctx context.Context, dest net.Destination) (*transport.Link, error) {
			// emulate what DefaultRouter.Dispatch does, and mutate something on the context
			ob := session.OutboundsFromContext(ctx)[0]
			ob.Target = dest
			return websiteDownlink, nil
		},
	}

	muxServerUplink, muxServerDownlink := newLinkPair()
	_, err := mux.NewServerWorker(serverCtx, &dispatcher, muxServerUplink)
	common.Must(err)

	client, err := mux.NewClientWorker(*muxServerDownlink, mux.ClientStrategy{})
	common.Must(err)

	clientCtx := session.ContextWithOutbounds(context.Background(), []*session.Outbound{{
		Target: net.TCPDestination(net.DomainAddress("www.example.com"), 80),
	}})

	muxClientUplink, muxClientDownlink := newLinkPair()

	ok := client.Dispatch(clientCtx, muxClientUplink)
	if !ok {
		t.Error("failed to dispatch")
	}

	{
		b := buf.FromBytes([]byte("hello"))
		common.Must(muxClientDownlink.Writer.WriteMultiBuffer(buf.MultiBuffer{b}))
	}

	resMb, err := websiteUplink.Reader.ReadMultiBuffer()
	common.Must(err)
	res := resMb.String()
	if res != "hello" {
		t.Error("upload: ", res)
	}

	{
		b := buf.FromBytes([]byte("world"))
		common.Must(websiteUplink.Writer.WriteMultiBuffer(buf.MultiBuffer{b}))
	}

	resMb, err = muxClientDownlink.Reader.ReadMultiBuffer()
	common.Must(err)
	res = resMb.String()
	if res != "world" {
		t.Error("download: ", res)
	}

	outbounds := session.OutboundsFromContext(serverCtx)
	if outbounds[0] != originalOutbounds[0] {
		t.Error("outbound got reassigned: ", outbounds[0])
	}

	if outbounds[0].Target.Address != nil {
		t.Error("outbound target got leaked: ", outbounds[0].Target.String())
	}
}

// Regression test: ServerWorker.run() goroutine must exit when context is cancelled,
// even while blocked reading a mux frame. Previously it leaked because the select
// in run() only checked ctx.Done() once before entering the blocking handleFrame().
func TestServerWorkerContextCancellation(t *testing.T) {
	dispatcher := TestDispatcher{
		OnDispatch: func(ctx context.Context, dest net.Destination) (*transport.Link, error) {
			reader, writer := pipe.New(pipe.WithoutSizeLimit())
			return &transport.Link{Reader: reader, Writer: writer}, nil
		},
	}

	// Simulate a silent/stuck client: don't write anything to uplinkWriter.
	uplinkReader, _ := pipe.New(pipe.WithoutSizeLimit())
	_, downlinkWriter := pipe.New(pipe.WithoutSizeLimit())

	link := &transport.Link{
		Reader: uplinkReader,
		Writer: downlinkWriter,
	}

	ctx, cancel := context.WithCancel(context.Background())

	worker, err := mux.NewServerWorker(ctx, &dispatcher, link)
	if err != nil {
		t.Fatal("failed to create ServerWorker:", err)
	}
	_ = worker

	// Wait for the worker goroutine to start and block on reading.
	time.Sleep(100 * time.Millisecond)

	// Record goroutine count after worker is blocked.
	runtime.GC()
	runtime.GC()
	before := runtime.NumGoroutine()

	// Cancel the context. The worker goroutine should exit promptly.
	cancel()

	// Poll until goroutines decrease or timeout.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		after := runtime.NumGoroutine()
		if after < before {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("ServerWorker goroutine did not exit within 3 seconds after context cancellation. goroutine before=%d, after=%d (goroutine leak confirmed)", before, runtime.NumGoroutine())
}

// Regression test: when context is cancelled while the ServerWorker is
// actively dispatching a session, all goroutines must be cleaned up.
func TestServerWorkerContextCancellationWhileDispatching(t *testing.T) {
	dispatcher := TestDispatcher{
		OnDispatch: func(ctx context.Context, dest net.Destination) (*transport.Link, error) {
			reader, writer := pipe.New(pipe.WithoutSizeLimit())
			return &transport.Link{Reader: reader, Writer: writer}, nil
		},
	}

	uplinkReader, uplinkWriter := pipe.New(pipe.WithoutSizeLimit())
	_, downlinkWriter := pipe.New(pipe.WithoutSizeLimit())

	link := &transport.Link{
		Reader: uplinkReader,
		Writer: downlinkWriter,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker, err := mux.NewServerWorker(ctx, &dispatcher, link)
	if err != nil {
		t.Fatal("failed to create ServerWorker:", err)
	}
	_ = worker

	dest := net.TCPDestination(net.DomainAddress("www.example.com"), 80)
	writer := mux.NewWriter(1, dest, uplinkWriter, protocol.TransferTypeStream, [8]byte{})
	b := buf.New()
	b.WriteString("hello")
	common.Must(writer.WriteMultiBuffer(buf.MultiBuffer{b}))
	writer.Close()

	time.Sleep(200 * time.Millisecond)

	runtime.GC()
	runtime.GC()
	before := runtime.NumGoroutine()

	cancel()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		after := runtime.NumGoroutine()
		if after < before {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("goroutines did not exit within 5 seconds after context cancellation (goroutine leak). Before: %d, After: %d.", before, runtime.NumGoroutine())
}
