package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bidirekt/broker/internal"
)

// stays under the 10s Docker waits between SIGTERM and SIGKILL
const shutdownTimeout = 8 * time.Second

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := internal.Healthcheck(); err != nil {
			log.Fatalf("healthcheck: %v", err)
		}
		return
	}

	components, err := internal.Run()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	listenErr := make(chan error, 1)
	go func() { listenErr <- components.Server.Listen(components.ListenAddr) }()

	select {
	case err := <-listenErr:
		log.Fatalf("server: %v", err)
	case <-ctx.Done():
	}

	if err := components.Server.ShutdownWithTimeout(shutdownTimeout); err != nil {
		log.Printf("shutdown: %v", err)
	}
	components.Pool.Close()
}
