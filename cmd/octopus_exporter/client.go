package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const octopusGraphQL = "https://api.octopus.energy/v1/graphql/"

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

func doGraphQL(req gqlRequest, authToken string) (map[string]any, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, octopusGraphQL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		httpReq.Header.Set("Authorization", "JWT "+authToken)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	if errs, ok := result["errors"].([]any); ok && len(errs) > 0 {
		if e, ok := errs[0].(map[string]any); ok {
			return nil, fmt.Errorf("GraphQL error: %s", e["message"])
		}
		return nil, errors.New("GraphQL error")
	}
	return result, nil
}

func toSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
