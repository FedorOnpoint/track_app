package httpserver_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/httpserver"
)

func TestNewServerSetsDefensiveTimeouts(t *testing.T) {
	t.Parallel()

	server := httpserver.NewServer(":8080", http.NotFoundHandler())

	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		t.Fatalf("server timeouts must all be positive: read_header=%v read=%v write=%v idle=%v",
			server.ReadHeaderTimeout, server.ReadTimeout, server.WriteTimeout, server.IdleTimeout)
	}
}

func TestRunStopsServerWhenContextIsCancelled(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := httpserver.NewServer(listener.Addr().String(), http.NotFoundHandler())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- httpserver.Run(ctx, server, listener)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop within 2 seconds")
	}
}
