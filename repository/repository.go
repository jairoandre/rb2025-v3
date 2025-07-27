package repository

import (
	"math"
	"rb2025-v3/model"
	"sync"
	"time"
)

type Repository struct {
	Payments *sync.Map
}

func NewRepository() *Repository {
	payments := new(sync.Map)
	return &Repository{Payments: payments}
}

func (r *Repository) Add(payment model.Payment) {
	r.Payments.Store(payment.CorrelationID, payment)
}

func (r *Repository) GetSummary(from, to time.Time) model.SummaryResponse {
	var defaultSummary, fallbackSummary model.Summary
	var defaultTotal, fallbackTotal float64
	r.Payments.Range(func(key, value any) bool {
		payment := value.(model.Payment)
		if payment.RequestedAt.Before(from) || payment.RequestedAt.After(to) {
			return true
		}
		switch payment.Processor {
		case 0:
			defaultTotal += payment.Amount
			defaultSummary.TotalRequests += 1
		case 1:
			fallbackTotal += payment.Amount
			fallbackSummary.TotalRequests += 1
		}
		return true
	})
	defaultSummary.TotalAmount = math.Round(float64(defaultTotal)*100) / 100
	fallbackSummary.TotalAmount = math.Round(float64(fallbackTotal)*100) / 100
	return model.SummaryResponse{Default: defaultSummary, Fallback: fallbackSummary}
}

func (r *Repository) PurgePayments() {
	r.Payments.Clear()
}
