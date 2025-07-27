package handler

import (
	"log"
	"rb2025-v3/client"
	"rb2025-v3/model"
	"rb2025-v3/repository"
	"time"

	"github.com/mailru/easyjson"
	"github.com/valyala/fasthttp"
)

type Handler struct {
	Jobs       chan<- model.PaymentRequest
	Repository *repository.Repository
	Client     *client.Client
	OtherUrl   string
}

func NewHandler(jobs chan<- model.PaymentRequest, r *repository.Repository, c *client.Client, otherUrl string) *Handler {
	return &Handler{Jobs: jobs, Repository: r, Client: c, OtherUrl: otherUrl}
}

func (h *Handler) PostPayments(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
		return
	}

	var req model.PaymentRequest
	err := easyjson.Unmarshal(ctx.PostBody(), &req)
	if err != nil {
		ctx.Error("Bad Request", fasthttp.StatusBadRequest)
		return
	}

	select {
	case h.Jobs <- req:
		ctx.SetStatusCode(fasthttp.StatusCreated)
	default:
		ctx.SetStatusCode(fasthttp.StatusTooManyRequests)
	}

}

func (h *Handler) PurgePayments(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
		return
	}
	h.Repository.PurgePayments()
	ctx.SetStatusCode(fasthttp.StatusAccepted)
}

func (h *Handler) GetSummary(ctx *fasthttp.RequestCtx) {
	if !ctx.IsGet() {
		ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
		return
	}
	fromStr := string(ctx.QueryArgs().Peek("from"))
	toStr := string(ctx.QueryArgs().Peek("to"))
	single := string(ctx.QueryArgs().Peek("single"))
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		from = time.Now().UTC().Add(-24 * time.Hour)
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		to = time.Now().UTC()
	}
	summary := h.Repository.GetSummary(from, to)
	if single == "" && h.OtherUrl != "" {
		otherSummary, err := h.Client.GetOtherSummary(h.OtherUrl, fromStr, toStr)
		if err != nil {
			log.Printf("Error getting other summary: %v", err)
		} else {
			summary.Default.TotalAmount += otherSummary.Default.TotalAmount
			summary.Default.TotalRequests += otherSummary.Default.TotalRequests
			summary.Fallback.TotalAmount += otherSummary.Fallback.TotalAmount
			summary.Fallback.TotalRequests += otherSummary.Fallback.TotalRequests
		}
	}
	ctx.Response.Header.Set("Content-Type", "application/json")
	if _, err := easyjson.MarshalToWriter(&summary, ctx); err != nil {
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
	}
}
