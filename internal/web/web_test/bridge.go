// internal/web/web_test/bridge.go
package web_test

import (
	"context"
	"net"
	"testing"

	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/server" // Import your real service package
	"github.com/tomekjarosik/qivitals/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// getRealQiVitalsClient sets up a real gRPC server backed by in-memory storage
// and returns a client to talk to it.
func getRealQiVitalsClient(t *testing.T) (v1.QiVitalsServiceClient, *storage.MemorySensorStorage) {
	t.Helper()

	store := storage.NewMemorySensorStorage()
	realService := server.NewStatusMonitorService(store)

	l := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	v1.RegisterQiVitalsServiceServer(grpcServer, realService)

	// ✅ Start the server in a goroutine so DialContext can connect
	go func() {
		if err := grpcServer.Serve(l); err != nil && err != grpc.ErrServerStopped {
			t.Logf("grpc server error: %v", err)
		}
	}()

	// ✅ Gracefully stop the server when the test ends
	t.Cleanup(func() {
		grpcServer.Stop()
	})

	conn, err := grpc.DialContext(context.Background(), "",
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return l.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}

	return v1.NewQiVitalsServiceClient(conn), store
}
