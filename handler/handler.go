package handler

import (
	"log"
	"rb2025-v3/client"
	"rb2025-v3/model"

	"github.com/mailru/easyjson"
	"github.com/valyala/fasthttp"
)

type Handler struct {
	Jobs   chan<- model.PaymentRequest
	Client *client.Client
}

func NewHandler(jobs chan<- model.PaymentRequest, c *client.Client) *Handler {
	return &Handler{Jobs: jobs, Client: c}
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
	h.Client.PurgeOnDb()
	ctx.SetStatusCode(fasthttp.StatusAccepted)
}

func (h *Handler) GetSummary(ctx *fasthttp.RequestCtx) {
	if !ctx.IsGet() {
		ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
		return
	}
	fromStr := string(ctx.QueryArgs().Peek("from"))
	toStr := string(ctx.QueryArgs().Peek("to"))
	summary, err := h.Client.GetSummary(fromStr, toStr)
	if err != nil {
		log.Printf("Error get summary: %v", err)
		ctx.Error("Error get summary", fasthttp.StatusInternalServerError)
	}
	ctx.Response.Header.Set("Content-Type", "application/json")
	if _, err := easyjson.MarshalToWriter(&summary, ctx); err != nil {
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
	}
}
