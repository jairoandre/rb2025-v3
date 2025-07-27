package handler

import (
	"rb2025-v3/model"
	"rb2025-v3/repository"
	"time"

	"github.com/mailru/easyjson"
	"github.com/valyala/fasthttp"
)

type Handler struct {
	Jobs       chan<- model.PaymentRequest
	Repository *repository.Repository
}

func NewHandler(jobs chan<- model.PaymentRequest, repository *repository.Repository) *Handler {
	return &Handler{Jobs: jobs, Repository: repository}
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
	h.Repository.Purge()
	ctx.SetStatusCode(fasthttp.StatusAccepted)
}

func (h *Handler) GetSummary(ctx *fasthttp.RequestCtx) {
	if !ctx.IsGet() {
		ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
		return
	}
	fromStr := string(ctx.QueryArgs().Peek("from"))
	toStr := string(ctx.QueryArgs().Peek("to"))
	from, err1 := time.Parse(time.RFC3339Nano, fromStr)
	if err1 != nil {
		from = time.Now().UTC().Add(-24 * time.Hour)
	}
	to, err2 := time.Parse(time.RFC3339Nano, toStr)
	if err2 != nil {
		to = time.Now().UTC()
	}
	defaultSummary := h.Repository.GetSummary("default", from, to)
	fallbackSummary := h.Repository.GetSummary("fallback", from, to)
	summary := model.SummaryResponse{
		Default:  defaultSummary,
		Fallback: fallbackSummary,
	}
	ctx.Response.Header.Set("Content-Type", "application/json")
	if _, err := easyjson.MarshalToWriter(&summary, ctx); err != nil {
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
	}
}
