package worker

import (
	"log"
	"rb2025-v3/client"
	"rb2025-v3/model"
	"rb2025-v3/repository"
	"time"
)

type Worker struct {
	Repository   *repository.Repository
	Client       *client.Client
	Jobs         chan model.PaymentRequest
	NumWorkers   int
	Suspended    bool
	ProcessorUrl string
	Processor    string
	SuspendedCh  chan struct{}
}

func NewWorker(r *repository.Repository, c *client.Client, jobs chan model.PaymentRequest, numWorkers int) *Worker {
	return &Worker{
		Repository:   r,
		Client:       c,
		Jobs:         jobs,
		NumWorkers:   numWorkers,
		Suspended:    false,
		ProcessorUrl: c.DefaultUrl,
		Processor:    "default",
		SuspendedCh:  make(chan struct{}),
	}

}

func (w *Worker) handleEvent(evt model.PaymentRequest) {
	requestedAt := time.Now().UTC()
	requestedAtStr := requestedAt.Format(time.RFC3339)
	paymentEvent := model.PaymentEvent{
		CorrelationID: evt.CorrelationID,
		Amount:        evt.Amount,
		RequestedAt:   requestedAtStr,
	}
	if w.Client.PostJSON(w.ProcessorUrl, paymentEvent) {
		w.Repository.Save(paymentEvent, requestedAt, w.Processor)
	} else {
		time.Sleep(100 * time.Millisecond)
		w.Jobs <- evt
	}
}

func (w *Worker) worker() {
	for {
		if w.Suspended {
			<-w.SuspendedCh
		}
		evt := <-w.Jobs
		w.handleEvent(evt)
	}
}

func (w *Worker) Start() {

	for i := 0; i < w.NumWorkers; i += 1 {
		go w.worker()
	}

	go func() {
		for {
			health, err := w.Client.ServiceHealth()
			if err != nil {
				time.Sleep(500 * time.Millisecond)
			}
			wasSuspended := w.Suspended
			w.Suspended = false
			if health.DefaultHealth && health.FallbackHealth {
				if health.DefaultMinResponse < health.FallbackMinResponse {
					w.ProcessorUrl = w.Client.DefaultUrl
					w.Processor = "default"
				} else {
					w.ProcessorUrl = w.Client.FallbackUrl
					w.Processor = "fallback"
				}
			} else if health.DefaultHealth {
				w.ProcessorUrl = w.Client.DefaultUrl
				w.Processor = "default"
			} else if health.FallbackHealth {
				w.ProcessorUrl = w.Client.FallbackUrl
				w.Processor = "fallback"
			} else {
				if !wasSuspended {
					log.Println("Suspend jobs")
				}
				w.Suspended = true
			}
			if wasSuspended && !w.Suspended {
				log.Println("Resume jobs")
				close(w.SuspendedCh)
				w.SuspendedCh = make(chan struct{})
			}
			time.Sleep(time.Duration(health.NextCheck+50) * time.Millisecond)
		}
	}()

}
