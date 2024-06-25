package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	client "github.com/justEngineer/go-yandex-personal-gofermart/internal/http/client"

	async "github.com/justEngineer/go-yandex-personal-gofermart/internal/async"
	database "github.com/justEngineer/go-yandex-personal-gofermart/internal/database"
	config "github.com/justEngineer/go-yandex-personal-gofermart/internal/http/server/config"
	server "github.com/justEngineer/go-yandex-personal-gofermart/internal/http/server/handlers"
	middleware "github.com/justEngineer/go-yandex-personal-gofermart/internal/http/server/middleware"
	logger "github.com/justEngineer/go-yandex-personal-gofermart/internal/logger"
)

func main() {
	cfg := config.Parse()
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	dbConnecton, err := database.New(ctx, &cfg)
	if err != nil {
		log.Printf("Database connection failed %s", err)
	} else {
		defer dbConnecton.CloseConnections()
	}

	appLogger, err := logger.New(cfg.LogLevel)
	if err != nil {
		log.Fatalf("Logger wasn't initialized due to %s", err)
	}

	AuthMiddleware := middleware.New(&cfg, appLogger, dbConnecton)
	ServerHandler := server.New(&cfg, appLogger, dbConnecton)

	accuralClient := client.New(cfg.AccuralEndpoint)
	workers := async.NewWorkerPool(accuralClient, dbConnecton)
	go func() {
		workers.Execute()
	}()

	port := strings.Split(cfg.Endpoint, ":")
	log.Fatal(http.ListenAndServe(":"+port[1], ServerHandler.GetRouter(AuthMiddleware)))

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	<-signalChannel
}
