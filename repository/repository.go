package repository

import (
	"context"
	"log"
	"math"
	"rb2025-v3/model"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	Pool *pgxpool.Pool
}

func New(dbUri string) *Repository {
	pool, err := pgxpool.New(context.Background(), dbUri)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	return &Repository{
		Pool: pool,
	}
}

func (r *Repository) Close() {
	r.Pool.Close()
}

func (r *Repository) SavePayment(p model.Payment) error {
	_, err := r.Pool.Exec(context.Background(),
		`INSERT INTO payments (correlation_id, amount, requested_at, processor) VALUES ($1, $2, $3, $4)`,
		p.CorrelationID,
		p.Amount,
		p.RequestedAt,
		p.Processor,
	)
	return err
}

func (r *Repository) Purge() error {
	_, err := r.Pool.Exec(context.Background(),
		`DELETE FROM payments`,
	)
	return err
}

func (r *Repository) GetSummary(from, to time.Time) (model.SummaryResponse, error) {
	rows, err := r.Pool.Query(context.Background(),
		`SELECT processor, sum(amount), count(*) FROM payments WHERE requested_at BETWEEN $1 AND $2 GROUP BY processor`,
		from,
		to,
	)
	if err != nil {
		return model.SummaryResponse{}, err
	}
	defer rows.Close()

	var summary model.SummaryResponse
	for rows.Next() {
		var processor int
		var totalAmount float64
		var totalRequests int
		if err := rows.Scan(&processor, &totalAmount, &totalRequests); err != nil {
			return model.SummaryResponse{}, err
		}
		switch processor {
		case 0:
			summary.Default.TotalAmount = math.Round(totalAmount*100) / 100
			summary.Default.TotalRequests = totalRequests
		case 1:

			summary.Fallback.TotalAmount = math.Round(totalAmount*100) / 100
			summary.Fallback.TotalRequests = totalRequests
		}
	}
	if rows.Err() != nil {
		return model.SummaryResponse{}, rows.Err()
	}
	return summary, nil
}
