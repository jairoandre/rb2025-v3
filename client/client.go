package client

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"rb2025-v3/model"
	"time"

	"github.com/mailru/easyjson"
)

type Client struct {
	DefaultUrl  string
	FallbackUrl string
	HealthUrl   string
	Client      *http.Client
}

func NewClient(defaultUrl, fallbackUrl, healthUrl string) *Client {
	transport := &http.Transport{
		MaxIdleConns:        2000, // Increase for high concurrency
		MaxIdleConnsPerHost: 2000, // Increase for high concurrency
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,  // Lower timeout for faster failover
			KeepAlive: 60 * time.Second, // Longer keepalive for connection reuse
		}).DialContext,
		// Optional: tune TLSHandshakeTimeout, ExpectContinueTimeout, etc.
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second, // Lower timeout for faster error returns
	}
	return &Client{
		DefaultUrl:  defaultUrl,
		FallbackUrl: fallbackUrl,
		HealthUrl:   healthUrl,
		Client:      client,
	}
}

// SendPayment tries default first, then fallback
func (c *Client) SendPayment(event model.PaymentEvent, serviceHealth model.ServiceHealthResponse) (int, error) {
	if serviceHealth.DefaultHealth && serviceHealth.FallbackHealth {
		firstUrl := c.DefaultUrl
		secondUrl := c.FallbackUrl
		firstProcessor := 0
		secondProcessor := 1
		if serviceHealth.FallbackMinResponse < serviceHealth.DefaultMinResponse {
			secondUrl = c.DefaultUrl
			firstProcessor = 1
			firstUrl = c.FallbackUrl
			secondProcessor = 0
		}
		if c.PostJSON(firstUrl, event) {
			return firstProcessor, nil
		}
		if c.PostJSON(secondUrl, event) {
			return secondProcessor, nil
		}
	} else if serviceHealth.DefaultHealth {
		if c.PostJSON(c.DefaultUrl, event) {
			return 0, nil
		}
	} else if serviceHealth.FallbackHealth {
		if c.PostJSON(c.FallbackUrl, event) {
			return 1, nil
		}
	}
	return -1, ErrBothFailed
}

var ErrBothFailed = &ProcessorError{"Both endpoints failed"}

type ProcessorError struct {
	Message string
}

func (e *ProcessorError) Error() string {
	return e.Message
}

// Internal POST logic
func (c *Client) PostJSON(url string, event model.PaymentEvent) bool {
	body, err := easyjson.Marshal(event)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return false
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/payments", url), bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Request creation error: %v", err)
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (c *Client) ServiceHealth() (model.ServiceHealthResponse, error) {
	resp, err := c.Client.Get(fmt.Sprintf("%s/health", c.HealthUrl))
	if err != nil {
		log.Printf("Service health error: %v", err)
		return model.ServiceHealthResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var serviceHealthResponse model.ServiceHealthResponse
		err = easyjson.UnmarshalFromReader(resp.Body, &serviceHealthResponse)
		if err != nil {
			return model.ServiceHealthResponse{}, err
		}
		return serviceHealthResponse, nil
	}
	return model.ServiceHealthResponse{}, errors.New("invalid service health response")
}
