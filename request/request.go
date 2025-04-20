package request

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"maps"

	"github.com/google/uuid"
	"github.com/infigaming-com/go-common/util"
	"go.uber.org/zap"
)

var (
	httpClient *http.Client
	once       sync.Once
)

type requestOption struct {
	lg                   *zap.Logger
	debugEnabled         bool
	queryParams          map[string]string
	requestHeaders       map[string]string
	requestBody          []byte
	signer               RequestSigner
	recorder             RequestRecorder
	signerKeys           any
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
		recorder:             nil,
		signerKeys:           nil,
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
		maps.Copy(option.queryParams, queryParams)
		return nil
	})
}

func WithRequestHeaders(requestHeaders map[string]string) Option {
	return optionFunc(func(option *requestOption) error {
		maps.Copy(option.requestHeaders, requestHeaders)
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
			option.lg.Error("[HTTP-REQUEST-ERROR: failed to marshal request body]",
				zap.Error(err),
				zap.Any("requestBody", requestBody),
			)
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		option.requestBody = jsonBody
		return nil
	})
}

func WithJsonAsQueryParamsAndRequestBody(requestBody any) Option {
	return optionFunc(func(option *requestOption) error {
		queryParams, requestBody, err := generateRequestData(requestBody)
		if err != nil {
			option.lg.Error("[HTTP-REQUEST-ERROR: failed to generate request data]",
				zap.Error(err),
				zap.Any("requestBody", requestBody),
			)
			return fmt.Errorf("failed to generate request data: %w", err)
		}
		option.queryParams = queryParams
		option.requestBody = requestBody
		return nil
	})
}

func WithRequestSigner(requestSigner RequestSigner, signerKeys any) Option {
	return optionFunc(func(option *requestOption) error {
		option.signer = requestSigner
		option.signerKeys = signerKeys
		return nil
	})
}

func WithRequestRecorder(requestRecord RequestRecorder) Option {
	return optionFunc(func(option *requestOption) error {
		option.recorder = requestRecord
		return nil
	})
}

func WithSlowRequestThreshold(slowRequestThreshold time.Duration) Option {
	return optionFunc(func(option *requestOption) error {
		if slowRequestThreshold <= 0 {
			option.lg.Error("[HTTP-REQUEST-ERROR: invalid slow request threshold]",
				zap.Duration("slowRequestThreshold", slowRequestThreshold),
			)
			return fmt.Errorf("invalid slow request threshold: %v", slowRequestThreshold)
		}
		option.slowRequestThreshold = slowRequestThreshold
		return nil
	})
}

func getHttpClient() *http.Client {
	once.Do(func() {
		httpClient = &http.Client{
			Timeout: 3 * time.Second, // timeout across all requests
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

		if option.recorder != nil {
			queryParams, _ := json.Marshal(option.queryParams)
			requestHeaders, _ := json.Marshal(option.requestHeaders)
			option.recorder(&RequestRecordData{
				Method:         method,
				Url:            requestUrl,
				QueryParams:    string(queryParams),
				RequestHeaders: string(requestHeaders),
				RequestBody:    string(option.requestBody),
				HttpStatusCode: httpStatusCode,
				ResponseBody:   string(responseBody),
				Duration:       time.Since(start).Milliseconds(),
			})

		}
	}()

	// sign the request
	if option.signer != nil {
		if err := option.signer(RequestSigningData{
			Method:         method,
			Url:            requestUrl,
			QueryParams:    option.queryParams,
			RequestHeaders: option.requestHeaders,
			RequestBody:    &option.requestBody,
		}, option.signerKeys); err != nil {
			option.lg.Error("[HTTP-REQUEST-ERROR: failed to sign request]",
				zap.Error(err),
				zap.String("method", method),
				zap.String("url", requestUrl),
				zap.Any("queryParams", option.queryParams),
				zap.Any("requestHeaders", option.requestHeaders),
				zap.ByteString("requestBody", option.requestBody),
			)
			return 0, nil, fmt.Errorf("failed to sign request: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, requestUrl, bytes.NewReader(option.requestBody))
	if err != nil {
		option.lg.Error("[HTTP-REQUEST-ERROR: failed to create request]",
			zap.Error(err),
			zap.String("method", method),
			zap.String("url", requestUrl),
			zap.ByteString("requestBody", option.requestBody),
		)
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}

	query := req.URL.Query()
	for k, v := range option.queryParams {
		query.Add(k, v)
	}
	req.URL.RawQuery = query.Encode()

	if option.correlationIdKey != "" && option.correlationId != "" {
		req.Header.Add(option.correlationIdKey, option.correlationId)
	} else {
		if correlationId, correlationIdErr := util.CorrelationIdFromCtx(ctx); correlationIdErr == nil {
			req.Header.Add(option.correlationIdKey, correlationId)
		} else {
			correlationId = uuid.New().String()
			req.Header.Add(option.correlationIdKey, correlationId)
		}
	}

	for k, v := range option.requestHeaders {
		req.Header.Add(k, v)
	}

	requestStart := time.Now()
	resp, err := getHttpClient().Do(req)
	if err == context.DeadlineExceeded {
		option.lg.Error("[HTTP-REQUEST-ERROR: request timeout]",
			zap.Error(err),
			zap.ByteString("requestBody", option.requestBody),
		)
		return 0, nil, fmt.Errorf("request timeout: %w", err)
	}
	if err != nil {
		option.lg.Error("[HTTP-REQUEST-ERROR: failed to send request]",
			zap.Error(err),
			zap.ByteString("requestBody", option.requestBody),
		)
		return 0, nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	requestDuration := time.Since(requestStart)

	httpStatusCode = resp.StatusCode

	responseBody, err = io.ReadAll(resp.Body)
	if err != nil {
		option.lg.Error("[HTTP-REQUEST-ERROR: failed to read response body]",
			zap.Error(err),
			zap.ByteString("requestBody", option.requestBody),
		)
		return 0, nil, fmt.Errorf("failed to read response body: %w", err)
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
	options = append(options, WithRequestHeaders(defaultHeader), WithJsonAsQueryParamsAndRequestBody(v))
	return Request(ctx, http.MethodPost, requestUrl, options...)
}
