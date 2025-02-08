package request

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/infigaming-com/go-common/util"
	"go.uber.org/zap"
)

var (
	httpClient *http.Client
	once       sync.Once
)

type requestSigner func(req *http.Request, apiKeyHeader, apiKey, signatureHeader, apiKeySecret string) error

type requestOption struct {
	lg                   *zap.Logger
	debugEnabled         bool
	queryParams          map[string]string
	requestHeaders       map[string]string
	requestBody          []byte
	signer               requestSigner
	apiKeyHeader         string
	apiKey               string
	signatureHeader      string
	apiKeySecret         string
	correlationIdKey     string
	correlationId        string
	slowRequestThreshold time.Duration
}

type Option interface {
	apply(option *requestOption) error
}

type optionFunc func(option *requestOption) error

func (f optionFunc) apply(option *requestOption) error {
	return f(option)
}

func defaultRequestOption() *requestOption {
	return &requestOption{
		lg:                   zap.L(),
		debugEnabled:         false,
		queryParams:          make(map[string]string),
		requestHeaders:       make(map[string]string),
		requestBody:          nil,
		signer:               nil,
		apiKeyHeader:         ApiKeyHeader,
		apiKey:               "",
		signatureHeader:      SignatureHeader,
		apiKeySecret:         "",
		correlationIdKey:     "X-Correlation-ID",
		correlationId:        "",
		slowRequestThreshold: 3 * time.Second,
	}
}

func WithLogger(lg *zap.Logger) Option {
	return optionFunc(func(option *requestOption) error {
		option.lg = lg
		return nil
	})
}

func WithDebugEnabled(debugEnabled bool) Option {
	return optionFunc(func(option *requestOption) error {
		option.debugEnabled = debugEnabled
		return nil
	})
}

func WithQueryParams(queryParams map[string]string) Option {
	return optionFunc(func(option *requestOption) error {
		for k, v := range queryParams {
			option.queryParams[k] = v
		}
		return nil
	})
}

func WithRequestHeaders(requestHeaders map[string]string) Option {
	return optionFunc(func(option *requestOption) error {
		for k, v := range requestHeaders {
			option.requestHeaders[k] = v
		}
		return nil
	})
}

func WithCorrelationId(correlationIdKey, correlationId string) Option {
	return optionFunc(func(option *requestOption) error {
		option.correlationIdKey = correlationIdKey
		option.correlationId = correlationId
		return nil
	})
}

func WithRequestBody(requestBody []byte) Option {
	return optionFunc(func(option *requestOption) error {
		option.requestBody = requestBody
		return nil
	})
}

func WithRequestBodyFromJson(requestBody any) Option {
	return optionFunc(func(option *requestOption) error {
		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return NewRequestError(
				ErrCodeInvalidRequestBody,
				"failed to marshal request body",
				err,
				nil,
				withRequestBody(option.requestBody),
			)
		}
		option.requestBody = jsonBody
		return nil
	})
}

func WithRequestSigner(requestSigner requestSigner, apiKeyHeader, apiKey, signatureHeader, apiKeySecret string) Option {
	return optionFunc(func(option *requestOption) error {
		option.signer = requestSigner
		option.apiKeyHeader = apiKeyHeader
		option.apiKey = apiKey
		option.signatureHeader = signatureHeader
		option.apiKeySecret = apiKeySecret
		return nil
	})
}

func WithSlowRequestThreshold(slowRequestThreshold time.Duration) Option {
	return optionFunc(func(option *requestOption) error {
		if slowRequestThreshold <= 0 {
			return NewRequestError(ErrCodeInvalidSlowRequestThreshold, "invalid slow request threshold", nil, nil)
		}
		option.slowRequestThreshold = slowRequestThreshold
		return nil
	})
}

func getHttpClient() *http.Client {
	once.Do(func() {
		httpClient = &http.Client{
			Timeout: 30 * time.Second, // timeout across all requests
		}
	})
	return httpClient
}

func Request(ctx context.Context, method string, requestUrl string, options ...Option) (httpStatusCode int, responseBody []byte, err error) {
	start := time.Now()

	option := defaultRequestOption()
	for _, opt := range options {
		if err := opt.apply(option); err != nil {
			return 0, nil, err
		}
	}

	defer func() {
		if err != nil {
			option.lg.Error("[HTTP-REQUEST-ERROR]",
				zap.Error(err),
				zap.String("method", method),
				zap.String("url", requestUrl),
				zap.Any("queryParams", option.queryParams),
				zap.Any("requestHeaders", option.requestHeaders),
				zap.ByteString("requestBody", option.requestBody),
				zap.Int("httpStatusCode", httpStatusCode),
				zap.ByteString("responseBody", responseBody),
				zap.Duration("duration", time.Since(start)),
			)
			return
		}

		if option.debugEnabled {
			option.lg.Debug("[HTTP-REQUEST-DEBUG]",
				zap.String("method", method),
				zap.String("url", requestUrl),
				zap.Any("queryParams", option.queryParams),
				zap.Any("requestHeaders", option.requestHeaders),
				zap.ByteString("requestBody", option.requestBody),
				zap.Int("httpStatusCode", httpStatusCode),
				zap.ByteString("responseBody", responseBody),
				zap.Duration("duration", time.Since(start)),
			)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, method, requestUrl, bytes.NewReader(option.requestBody))
	if err != nil {
		return 0, nil, NewRequestError(ErrCodeFailedToCreateRequest, "failed to create request", err, nil, withURL(requestUrl), withMethod(method), withRequestBody(option.requestBody))
	}

	query := req.URL.Query()
	for k, v := range option.queryParams {
		query.Add(k, v)
	}
	req.URL.RawQuery = query.Encode()

	var correlationId string
	if option.correlationIdKey != "" && option.correlationId != "" {
		req.Header.Add(option.correlationIdKey, option.correlationId)
	} else {
		correlationId, err = util.CorrelationIdFromCtx(ctx)
		if err != nil {
			correlationId = uuid.New().String()
		}
		req.Header.Add(option.correlationIdKey, correlationId)
	}

	for k, v := range option.requestHeaders {
		req.Header.Add(k, v)
	}

	// sign the request
	if option.signer != nil && option.apiKey != "" && option.apiKeySecret != "" {
		if err := option.signer(req, option.apiKeyHeader, option.apiKey, option.signatureHeader, option.apiKeySecret); err != nil {
			return 0, nil, NewRequestError(ErrCodeFailedToSignRequest, "failed to sign request", err, nil)
		}
	}

	requestStart := time.Now()
	resp, err := getHttpClient().Do(req)
	if err != nil {
		return 0, nil, NewRequestError(ErrCodeFailedToSendRequest, "failed to send request", err, nil, withMethod(method), withURL(requestUrl), withRequestBody(option.requestBody))
	}
	defer resp.Body.Close()
	requestDuration := time.Since(requestStart)

	httpStatusCode = resp.StatusCode

	responseBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, NewRequestError(ErrCodeFailedToReadResponseBody, "failed to read response body", err, nil)
	}

	if requestDuration > option.slowRequestThreshold {
		option.lg.Warn("[HTTP-REQUEST-SLOW]",
			zap.String("method", method),
			zap.String("url", requestUrl),
			zap.Any("queryParams", option.queryParams),
			zap.Any("requestHeaders", option.requestHeaders),
			zap.ByteString("requestBody", option.requestBody),
			zap.Int("httpStatusCode", httpStatusCode),
			zap.ByteString("responseBody", responseBody),
			zap.Duration("duration", requestDuration),
		)
	}

	return httpStatusCode, responseBody, nil
}

func Get(ctx context.Context, requestUrl string, options ...Option) (httpStatusCode int, responseBody []byte, err error) {
	return Request(ctx, http.MethodGet, requestUrl, options...)
}

func Post(ctx context.Context, requestUrl string, requestBody []byte, options ...Option) (httpStatusCode int, responseBody []byte, err error) {
	defaultHeader := map[string]string{"Content-Type": "application/json"}
	options = append(options, WithRequestHeaders(defaultHeader), WithRequestBody(requestBody))
	return Request(ctx, http.MethodPost, requestUrl, options...)
}

func PostJson(ctx context.Context, requestUrl string, v any, options ...Option) (httpStatusCode int, responseBody []byte, err error) {
	defaultHeader := map[string]string{"Content-Type": "application/json"}
	options = append(options, WithRequestHeaders(defaultHeader), WithRequestBodyFromJson(v))
	return Request(ctx, http.MethodPost, requestUrl, options...)
}
