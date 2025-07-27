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
	"strconv"
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

	defaultUrl := readEnv("DEFAULT_URL", "http://localhost:8001")
	fallbackUrl := readEnv("FALLBACK_URL", "http://localhost:8002")
	healthUrl := readEnv("HEALTH_URL", "http://localhost:9001")
	otherUrl := readEnv("OTHER_URL", "")
	numWorkers, _ := strconv.Atoi(readEnv("NUM_WORKERS", "2000"))
	defaultTolerance, _ := strconv.Atoi(readEnv("DEFAULT_TOLERANCE", "1500"))
	semaphoreSize, _ := strconv.Atoi(readEnv("SEMAPHORE_SIZE", "50"))
	jobsBufferSize, _ := strconv.Atoi(readEnv("JOBS_BUFFER_SIZE", "10000"))
	workerSleep, _ := strconv.Atoi(readEnv("WORKER_SLEEP", "50"))

	jobs := make(chan model.PaymentRequest, jobsBufferSize)
	r := repository.NewRepository()
	c := client.NewClient(defaultUrl, fallbackUrl, healthUrl)
	h := handler.NewHandler(jobs, r, c, otherUrl)
	w := worker.NewWorker(jobs, r, c, numWorkers, defaultTolerance, semaphoreSize, workerSleep)

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
