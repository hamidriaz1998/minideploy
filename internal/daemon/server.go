package daemon

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hamid/minideploy/internal/shared"
)

func Run(port int, stateDir string) error {
	state, err := NewStateManager(stateDir)
	if err != nil {
		return fmt.Errorf("init state: %w", err)
	}

	router := NewRouter(state)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		shared.Info("minideploy daemon v%s listening on %s", Version, addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			shared.Fatal("listen: %v", err)
		}
	}()

	<-quit
	shared.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("forced shutdown: %w", err)
	}

	shared.Info("daemon stopped")
	return nil
}
