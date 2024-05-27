package client

import (
	"errors"
	"net/http"
	"strings"

	models "github.com/justEngineer/go-yandex-personal-gofermart/internal/models"

	"github.com/go-resty/resty/v2"
)

const (
	GetOrderInfoURL   = "/api/orders/{number}"
	OrderIDBindingURL = "number"
)

var (
	ErrTooManyRequests = errors.New("too many requests")
	ErrNoContent       = errors.New("no content")
)

type AccrualClient struct {
	client   *resty.Client
	endpoint string
}

func New(endpoint string) *AccrualClient {
	return &AccrualClient{client: resty.New(), endpoint: endpoint}
}

func (c *AccrualClient) GetOrderInfo(orderID string) (models.AccrualInfo, error) {
	var accrual models.AccrualInfo
	response, err := c.client.R().
		SetResult(&accrual).
		SetRawPathParam(OrderIDBindingURL, orderID).
		Get(c.endpoint + GetOrderInfoURL)
	if response.StatusCode() == http.StatusTooManyRequests {
		return accrual, ErrTooManyRequests
	}
	if strings.Contains(response.Status(), http.StatusText(http.StatusNoContent)) {
		return accrual, ErrNoContent
	}
	if err != nil {
		return accrual, err
	}
	return accrual, nil
}
