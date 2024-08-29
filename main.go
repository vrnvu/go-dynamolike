package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/vrnvu/go-dynamolike/internal/discovery"
	"github.com/vrnvu/go-dynamolike/internal/server"
)

const shortUsage = `Usage of go-dynamolike:

	$ go-dynamolike --network <network-name>

Flags:
	--network <network-name>  (REQUIRED)
		Specify the Docker network name to use for service discovery.
		This flag determines which network the program will scan to find MinIO instances.

Example:
	$ go-dynamolike --network dynamolike-network

Note: The --network flag is mandatory. The program will not run without it.
`

func init() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

func main() {
	if len(os.Args) == 1 {
		fmt.Print(shortUsage)
		return
	}
	log.SetFlags(0)
	var (
		networkFlag = flag.String("network", "", "Docker network name")
	)
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), shortUsage)
	}
	flag.Parse()
	if *networkFlag == "" {
		fmt.Println("Error: --network flag is required")
		flag.Usage()
		return
	}
	run(*networkFlag)
}

func run(network string) {
	// TODO we are going to sleep for the first version so the partition are fixed
	time.Sleep(3 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		slog.Error("Error creating Docker client", "error", err)
		cancel()
		return
	}
	defer cli.Close()

	registry := discovery.NewServiceRegistry(ctx, cli, network)
	if err := registry.PollNetwork(); err != nil {
		slog.Error("Error in Minio discovery", "error", err)
		return
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ticker.C:
				if err := registry.PollNetwork(); err != nil {
					slog.Error("Error in Minio discovery", "error", err)
					cancel()
					return
				}
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	// Start the server
	port := 3000
	server := server.NewServer(port, registry)

	slog.Info("Server is running", "port", port)
	go func() {
		err := server.Server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			cancel()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		slog.Info("Shutting down...")
	case <-ctx.Done():
		slog.Info("Shutting down due to error...")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	err = server.Server.Shutdown(shutdownCtx)
	if err != nil {
		slog.Error("Error during shutdown", "error", err)
	}
}
