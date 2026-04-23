package main

import "errors"

var errTokenExpired = errors.New("token expired")

func getKrakenToken(apiKey string) (string, error) {
	result, err := doGraphQL(gqlRequest{
		Variables: map[string]any{"apikey": apiKey},
		Query: `mutation krakenTokenAuthentication($apikey: String!) {
			obtainKrakenToken(input: {APIKey: $apikey}) {
				token
			}
		}`,
	}, "")
	if err != nil {
		return "", err
	}
	token, ok := result["data"].(map[string]any)["obtainKrakenToken"].(map[string]any)["token"].(string)
	if !ok {
		return "", errors.New("token not found in response")
	}
	return token, nil
}
