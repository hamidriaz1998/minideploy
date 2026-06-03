package daemon

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Run(port int, stateDir string) error {
	state, err := NewStateManager(stateDir)
	if err != nil {
		return fmt.Errorf("init state: %w", err)
	}

	if len(state.GetAPIKeys()) == 0 {
		raw, hash, err := GenerateAPIKey()
		if err != nil {
			return fmt.Errorf("generate api key: %w", err)
		}
		if err := state.AddAPIKey(hash); err != nil {
			return fmt.Errorf("store api key: %w", err)
		}
		log.Printf("!!! No API key configured. Generated one-time key:")
		log.Printf("!!!   %s", raw)
		log.Printf("!!! Set this as MINIDEPLOY_API_KEY or in .deploy.yml server.api_key")
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
		log.Printf("minideploy daemon v%s listening on %s", Version, addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("forced shutdown: %w", err)
	}

	log.Println("daemon stopped")
	return nil
}
