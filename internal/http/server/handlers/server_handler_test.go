package server

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"io"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	config "github.com/justEngineer/go-yandex-personal-gofermart/internal/http/server/config"
	middleware "github.com/justEngineer/go-yandex-personal-gofermart/internal/http/server/middleware"
	logger "github.com/justEngineer/go-yandex-personal-gofermart/internal/logger"
	mocks "github.com/justEngineer/go-yandex-personal-gofermart/internal/mocks"
	models "github.com/justEngineer/go-yandex-personal-gofermart/internal/models"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func testRequest(t *testing.T, ts *httptest.Server, method, path string, headers map[string]string, body io.Reader) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, body)
	require.NoError(t, err)

	req.Header.Set("Accept-Encoding", "identity")

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp, string(respBody)
}

func TestValidateOrder(t *testing.T) {
	tests := []struct {
		name   string
		order  string
		result bool
	}{
		{
			name:   "not integer order",
			order:  "SomeText",
			result: false,
		},
		{
			name:   "negative order",
			order:  "-5",
			result: false,
		},
		{
			name:   "correct order with odd digit quantity",
			order:  "12345678903",
			result: true,
		},
		{
			name:   "incorrect order with odd digit quantity",
			order:  "124",
			result: false,
		},
		{
			name:   "correct order with even digit quantity",
			order:  "12345678903",
			result: true,
		},
		{
			name:   "incorrect order with even digit quantity",
			order:  "3743",
			result: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateOrderID(tt.order)
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestRegisterRoute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	storage := mocks.NewMockStorage(ctrl)
	var cfg config.ServerConfig
	appLogger, err := logger.New(cfg.LogLevel)
	if err != nil {
		log.Fatalf("Logger wasn't initialized due to %s", err)
	}

	AuthMiddleware := middleware.New(&cfg, appLogger, storage)
	ServerHandler := New(&cfg, appLogger, storage)

	testServer := httptest.NewServer(ServerHandler.GetRouter(AuthMiddleware))
	defer testServer.Close()

	testCases := []struct {
		testName        string
		methodName      string
		targetURL       string
		body            func() io.Reader
		test            func(t *testing.T)
		expectedCode    int
		expectedMessage string
	}{
		{
			testName:        "Should return a validation error due to missing body",
			methodName:      "POST",
			targetURL:       "/api/user/register",
			expectedCode:    http.StatusBadRequest,
			expectedMessage: "Error while unmarshaling data unexpected end of JSON input\n",
		},
		{
			testName:   "Should return a validation error due to missing user login",
			methodName: "POST",
			targetURL:  "/api/user/register",
			body: func() io.Reader {
				Password := "123"
				data, _ := json.Marshal(models.UserAuthData{Password: Password})
				return bytes.NewBuffer(data)
			},
			expectedCode:    http.StatusBadRequest,
			expectedMessage: "Request doesn't contain login or password\n",
		},
		{
			testName:   "Should return a validation error due to missing user password",
			methodName: "POST",
			targetURL:  "/api/user/register",
			body: func() io.Reader {
				Login := "user"
				data, _ := json.Marshal(models.UserAuthData{Login: Login})
				return bytes.NewBuffer(data)
			},
			expectedCode:    http.StatusBadRequest,
			expectedMessage: "Request doesn't contain login or password\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			var body io.Reader

			if tc.body != nil {
				body = tc.body()
			}

			if tc.test != nil {
				tc.test(t)
			}

			res, mes := testRequest(
				t,
				testServer,
				tc.methodName,
				tc.targetURL,
				map[string]string{"Content-Type": "application/json"},
				body,
			)
			res.Body.Close()

			assert.Equal(t, tc.expectedCode, res.StatusCode)
			assert.Equal(t, tc.expectedMessage, mes)
		})
	}
}
