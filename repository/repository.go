package repository

import (
	"context"
	"fmt"
	"log"
	"math"
	"rb2025-v3/model"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	amountsKey      = "amounts"
	correlationsKey = "correlations"
)

type Repository struct {
	RedisClient redis.Client
}

func NewRepository(redisUrl string) *Repository {
	return &Repository{
		RedisClient: *redis.NewClient(&redis.Options{
			Addr:     redisUrl,
			Password: "",
			DB:       0,
		}),
	}
}

func getKey(processor, category string) string {
	return fmt.Sprintf("payments:%s:%s", processor, category)
}

func (r *Repository) Purge() {
	ctx := context.Background()
	if err := r.RedisClient.FlushAll(ctx).Err(); err != nil {
		log.Println("Error on purge redis")
	}
}

func (r *Repository) Save(paymentEvent model.PaymentEvent, requestedAt time.Time, processor string) {
	ctx := context.Background()
	pipeline := r.RedisClient.Pipeline()
	pipeline.HSet(ctx, getKey(processor, amountsKey), paymentEvent.CorrelationID, paymentEvent.Amount)
	pipeline.ZAdd(ctx, getKey(processor, correlationsKey), redis.Z{
		Score:  float64(requestedAt.UnixMilli()),
		Member: paymentEvent.CorrelationID,
	})
	if _, err := pipeline.Exec(ctx); err != nil {
		log.Println("Error saving on redis", err)
	}
}

func (r *Repository) GetSummary(processor string, from, to time.Time) model.Summary {
	ctx := context.Background()
	var summary model.Summary
	ids, _ := r.RedisClient.ZRangeByScore(ctx, getKey(processor, correlationsKey), &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", from.UnixMilli()),
		Max: fmt.Sprintf("%d", to.UnixMilli()),
	}).Result()
	if len(ids) == 0 {
		return summary
	}

	amounts, err := r.RedisClient.HMGet(ctx, getKey(processor, amountsKey), ids...).Result()
	if err != nil {
		log.Printf("Error on summary: %v", err)
	}
	for _, amount := range amounts {
		if amountStr, ok := amount.(string); ok {
			if amountFloat, err := strconv.ParseFloat(amountStr, 64); err == nil {
				summary.TotalAmount += amountFloat
				summary.TotalRequests += 1
			}
		}
	}
	summary.TotalAmount = math.Round(summary.TotalAmount*100) / 100
	return summary
}
