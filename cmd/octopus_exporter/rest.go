package main

import (
	"encoding/json"
	"net/http"
	"net/url"
)

var octopusREST = "https://api.octopus.energy"

func doREST(path string, params url.Values) (map[string]any, error) {
	u := octopusREST + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	raw, err := executeWithRetry(func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(apiKey, "")
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}
