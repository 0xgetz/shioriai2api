// Command shioriai2api runs an OpenAI-compatible HTTP proxy in front of
// shiori.ai using your own logged-in account.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xgetz/shioriai2api/config"
	"github.com/0xgetz/shioriai2api/handlers"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	convs := handlers.NewConvStore(cfg.ConversationTTL, cfg.MaxConvs)
	h := handlers.New(cfg, handlers.NewPool(cfg.Accounts, cfg.BaseURL, cfg.Cookie, cfg.Proxy), convs)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /v1/models", h.Auth(h.ListModels))
	mux.HandleFunc("POST /v1/chat/completions", h.Auth(h.ChatCompletion))

	sweepCtx, stopSweep := context.WithCancel(context.Background())
	defer stopSweep()
	go convs.SweepLoop(sweepCtx)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("shioriai2api listening on :%s", cfg.Port)
		log.Printf("  upstream : %s", cfg.BaseURL)
		log.Printf("  accounts : %d", len(cfg.Accounts))
		log.Printf("  default  : %s", cfg.DefaultModel)
		log.Printf("  conv ttl : %s", cfg.ConversationTTL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")
	stopSweep()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
}
