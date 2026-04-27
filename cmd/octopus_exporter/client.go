package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	octopusGraphQL   = "https://api.octopus.energy/v1/graphql/"
	httpClient       = &http.Client{Timeout: 15 * time.Second}
	rateLimitRetries prometheus.Counter
)

type gqlRequest struct {
	OperationName string         `json:"operationName,omitempty"`
	Variables     map[string]any `json:"variables"`
	Query         string         `json:"query"`
}

// jsonFloat unmarshals both JSON numbers and quoted strings into float64.
type jsonFloat float64

func (f *jsonFloat) UnmarshalJSON(data []byte) error {
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = jsonFloat(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = jsonFloat(n)
	return nil
}

// executeWithRetry executes an HTTP request, retrying on 429 with exponential
// backoff (honouring Retry-After when present). Returns the raw response body.
func executeWithRetry(makeReq func() (*http.Request, error)) ([]byte, error) {
	const maxRetries = 5
	backoff := 30 * time.Second
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := makeReq()
		if err != nil {
			return nil, err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			if attempt == maxRetries {
				return nil, errors.New("rate limited: max retries exceeded")
			}
			if rateLimitRetries != nil {
				rateLimitRetries.Inc()
			}
			wait := backoff
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil {
					wait = time.Duration(secs) * time.Second
				}
			}
			log.Printf("rate limited; retrying in %v (attempt %d/%d)", wait, attempt+1, maxRetries)
			time.Sleep(wait)
			backoff *= 2
			continue
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			snippet := strings.TrimSpace(string(raw))
			if len(snippet) > 200 {
				snippet = snippet[:200]
			}
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet)
		}
		return raw, nil
	}
	return nil, errors.New("rate limited: max retries exceeded")
}

func doGraphQL(req gqlRequest, authToken string) (map[string]any, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	raw, err := executeWithRetry(func() (*http.Request, error) {
		httpReq, err := http.NewRequest(http.MethodPost, octopusGraphQL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if authToken != "" {
			httpReq.Header.Set("Authorization", "JWT "+authToken)
		}
		return httpReq, nil
	})
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	if errs, ok := result["errors"].([]any); ok && len(errs) > 0 {
		if e, ok := errs[0].(map[string]any); ok {
			msg, _ := e["message"].(string)
			if strings.Contains(msg, "JWT") && strings.Contains(msg, "expired") {
				return nil, errTokenExpired
			}
			return nil, fmt.Errorf("GraphQL error: %s", msg)
		}
		return nil, errors.New("GraphQL error")
	}
	return result, nil
}

func toSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
