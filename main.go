package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"rb2025-v3/client"
	"rb2025-v3/handler"
	"rb2025-v3/model"
	"rb2025-v3/repository"
	"rb2025-v3/worker"
	"syscall"
	"time"

	"github.com/valyala/fasthttp"
)

func readEnv(envName string, defaultValue string) string {
	envValue, exists := os.LookupEnv(envName)
	if exists {
		return envValue
	}
	return defaultValue
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	jobs := make(chan model.PaymentRequest, 100000)
	redisUrl := readEnv("REDIS_URL", "localhost:6379")
	defaultUrl := readEnv("DEFAULT_URL", "http://localhost:8001")
	fallbackUrl := readEnv("FALLBACK_URL", "http://localhost:8002")
	healthUrl := readEnv("HEALTH_URL", "http://localhost:9001")

	r := repository.NewRepository(redisUrl)
	c := client.NewClient(defaultUrl, fallbackUrl, healthUrl)
	h := handler.NewHandler(jobs, r)
	w := worker.NewWorker(r, c, jobs, 1000)

	server := &fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			switch string(ctx.Path()) {
			case "/payments":
				h.PostPayments(ctx)
			case "/payments-summary":
				h.GetSummary(ctx)
			case "/purge-payments":
				h.PurgePayments(ctx)
			default:
				ctx.SetStatusCode(fasthttp.StatusNotFound)
			}
		},
	}

	port := readEnv("SERVER_PORT", "9999")

	go func() {
		log.Printf("Listening on port %s", port)
		if err := server.ListenAndServe(fmt.Sprintf(":%s", port)); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	w.Start()

	<-ctx.Done()
	log.Println("Shutdown signal received")
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
	log.Println("Application closed")
}
