package api

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/api/handlers"
	"github.com/BGYKanishka/baleen-engine/internal/cli"
)

func StartDaemonServer(ctx cli.EngineContext, token string) {
	lastActive = time.Now()
	go func() {
		for approval := range ctx.ApprovalChan {
			ctx.PendingApproval.Store(approval)
		}
	}()

	mux := http.NewServeMux()

	// Wire up all route handlers, injecting shared dependencies
	mux.HandleFunc("/api/health", withHeartbeat(handlers.Health()))
	mux.HandleFunc("/api/peers", withHeartbeat(handlers.Peers(ctx)))
	mux.HandleFunc("/api/peers/", withHeartbeat(handlers.Peers(ctx)))
	mux.HandleFunc("/api/images", withHeartbeat(handlers.Images()))
	mux.HandleFunc("/api/ledger", withHeartbeat(handlers.History(ctx)))
	mux.HandleFunc("/api/push", withHeartbeat(handlers.Dispatch(ctx)))
	mux.HandleFunc("/api/stream", withHeartbeat(handlers.Stream()))
	mux.HandleFunc("/api/pending", withHeartbeat(handlers.Pending(ctx)))
	mux.HandleFunc("/api/approve", withHeartbeat(handlers.Approve(ctx)))
	mux.HandleFunc("/api/reject", withHeartbeat(handlers.Reject(ctx)))

	handler := corsAndAuthMiddleware(mux, token)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start daemon listener: %v\n", err)
		os.Exit(1)
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port
	fmt.Printf(`{"status": "ready", "port": %d}`+"\n", actualPort)
	os.Stdout.Sync()

	// Monitor idle time and exit if the extension closed or crashed
	go func() {
		for {
			time.Sleep(5 * time.Second)
			if idleTime() > 2*time.Minute {
				os.Exit(0)
			}
		}
	}()

	http.Serve(listener, handler)
}

// wraps any handler to update the idle timer on each request
func withHeartbeat(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		UpdateHeartbeat()
		next(w, r)
	}
}
