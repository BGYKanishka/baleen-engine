package api

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/api/handlers"
	"github.com/BGYKanishka/baleen-engine/internal/cli"
)

// StartDaemonServer starts the HTTP API server for the daemon, listening on a random localhost port.
func StartDaemonServer(ctx cli.EngineContext, token string, stopCh chan<- struct{}, apiPortCh chan<- int) {
	lastActive = time.Now()
	go func() {
		for approval := range ctx.ApprovalChan {
			ctx.PendingApproval.Store(approval)
		}
	}()

	//auto-shutdown if no client has pinged the API in 5 minutes AND no active transfers.
	go func() {
		for {
			time.Sleep(10 * time.Second)
			if idleTime() > 5*time.Minute && ctx.ActiveTransfers.Load() == 0 {
				slog.Info("Daemon is idle and abandoned (no active transfers), shutting down")
				select {
				case stopCh <- struct{}{}:
				default:
				}
				return
			}
		}
	}()

	mux := http.NewServeMux()

	// Wire up all route handlers, injecting shared dependencies
	mux.HandleFunc("GET /api/health", withHeartbeat(handlers.Health()))
	mux.HandleFunc("GET /api/peers", withHeartbeat(handlers.Peers(ctx)))
	mux.HandleFunc("POST /api/peers", withHeartbeat(handlers.Peers(ctx)))
	mux.HandleFunc("DELETE /api/peers/", withHeartbeat(handlers.Peers(ctx)))
	mux.HandleFunc("GET /api/ledger", withHeartbeat(handlers.History(ctx)))
	mux.HandleFunc("POST /api/push", withHeartbeat(handlers.Dispatch(ctx)))
	mux.HandleFunc("GET /api/stream", withHeartbeat(handlers.Stream()))
	mux.HandleFunc("GET /api/pending", withHeartbeat(handlers.Pending(ctx)))
	mux.HandleFunc("POST /api/approve", withHeartbeat(handlers.Approve(ctx)))
	mux.HandleFunc("POST /api/reject", withHeartbeat(handlers.Reject(ctx)))
	mux.HandleFunc("POST /api/gc", withHeartbeat(handlers.GC(ctx)))
	mux.HandleFunc("GET /api/logs", withHeartbeat(handlers.Logs()))
	mux.HandleFunc("POST /api/logs/clean", withHeartbeat(handlers.CleanLogs()))
	mux.HandleFunc("POST /api/transfer/pause", withHeartbeat(handlers.Pause()))
	mux.HandleFunc("POST /api/transfer/resume", withHeartbeat(handlers.Resume()))
	mux.HandleFunc("POST /api/transfer/cancel", withHeartbeat(handlers.Cancel()))
	mux.HandleFunc("GET /api/node/name", withHeartbeat(handlers.NodeName(ctx)))
	mux.HandleFunc("POST /api/node/name", withHeartbeat(handlers.NodeName(ctx)))
	mux.HandleFunc("GET /api/network/settings", withHeartbeat(handlers.NetworkSettings(ctx)))
	mux.HandleFunc("POST /api/network/settings", withHeartbeat(handlers.NetworkSettings(ctx)))
	mux.HandleFunc("GET /api/transfer/settings", withHeartbeat(handlers.TransferSettings()))
	mux.HandleFunc("POST /api/transfer/settings", withHeartbeat(handlers.TransferSettings()))
	mux.HandleFunc("POST /api/settings/reset", withHeartbeat(handlers.ResetSettings(ctx)))

	// Stop endpoint: UI/CLI call this when the user clicks the Stop button.
	mux.HandleFunc("POST /api/stop", withHeartbeat(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"stopping"}`))
		// Flush first, then signal main.
		go func() {
			time.Sleep(100 * time.Millisecond)
			select {
			case stopCh <- struct{}{}:
			default:
			}
		}()
	}))

	handler := corsAndAuthMiddleware(mux, token)

	// Bind to a random localhost port — this is the HTTP API port (not the TLS P2P port).
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		slog.Error("failed to start daemon listener", "error", err)
		os.Exit(1)
	}

	apiPort := listener.Addr().(*net.TCPAddr).Port

	// Print the API port to stdout in JSON format so that the CLI can read it.
	select {
	case apiPortCh <- apiPort:
	default:
	}

	fmt.Printf(`{"status": "ready", "port": %d}`+"\n", apiPort)
	os.Stdout.Sync()

	http.Serve(listener, handler)
}

// wraps any handler to update the idle timer on each request.
func withHeartbeat(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		UpdateHeartbeat()
		next(w, r)
	}
}
