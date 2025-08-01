package worker

import (
	"log"
	"rb2025-v3/client"
	"rb2025-v3/model"
	"rb2025-v3/repository"
	"time"
)

type Worker struct {
	Jobs             chan model.PaymentRequest
	Repository       *repository.Repository
	Client           *client.Client
	NumWorkers       int
	DefaultTolerance int
	Suspended        bool
	ProcessorUrl     string
	Processor        int
	WorkerSleep      int
	SuspendedCh      chan struct{}
	Semaphore        chan struct{}
}

func NewWorker(jobs chan model.PaymentRequest, r *repository.Repository, c *client.Client, numWorkers, defaultTolerance, semaphoreSize, workerSleep int) *Worker {
	return &Worker{
		Jobs:         jobs,
		Repository:   r,
		Client:       c,
		NumWorkers:   numWorkers,
		Suspended:    false,
		ProcessorUrl: c.DefaultUrl,
		Processor:    0,
		SuspendedCh:  make(chan struct{}),
		Semaphore:    make(chan struct{}, semaphoreSize),
	}
}

func (w *Worker) handleEvent(evt model.PaymentRequest) {
	w.Semaphore <- struct{}{}
	requestedAt := time.Now().UTC()
	requestedAtStr := requestedAt.Format(time.RFC3339Nano)
	paymentEvent := model.PaymentEvent{
		CorrelationID: evt.CorrelationID,
		Amount:        evt.Amount,
		RequestedAt:   requestedAtStr,
	}
	if w.Client.PostJSON(w.ProcessorUrl, paymentEvent) {
		payment := model.Payment{
			CorrelationID: evt.CorrelationID,
			Amount:        evt.Amount,
			Processor:     w.Processor,
			RequestedAt:   requestedAt,
		}
		w.Repository.Add(payment)
	} else {
		w.Jobs <- evt
	}
	<-w.Semaphore
	time.Sleep(time.Duration(w.WorkerSleep) * time.Millisecond)
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
				if health.DefaultMinResponse < (health.FallbackMinResponse + 1000) {
					w.ProcessorUrl = w.Client.DefaultUrl
					w.Processor = 0
				} else {
					w.ProcessorUrl = w.Client.FallbackUrl
					w.Processor = 1
				}
			} else if health.DefaultHealth {
				w.ProcessorUrl = w.Client.DefaultUrl
				w.Processor = 0
			} else if health.FallbackHealth {
				w.ProcessorUrl = w.Client.FallbackUrl
				w.Processor = 1
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
