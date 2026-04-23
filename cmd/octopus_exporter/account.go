package main

import "errors"

func getAccountBalance(token string) (float64, error) {
	result, err := doGraphQL(gqlRequest{
		Query: `{ viewer { accounts { ... on AccountType { balance } } } }`,
	}, token)
	if err != nil {
		return 0, err
	}

	accounts, _ := result["data"].(map[string]any)["viewer"].(map[string]any)["accounts"].([]any)
	for _, a := range accounts {
		if bal, ok := a.(map[string]any)["balance"].(float64); ok {
			return bal, nil
		}
	}

	return 0, errors.New("balance not found in response")
}
