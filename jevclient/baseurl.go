package jevclient

import "github.com/rshade/go-decide/internal/clientkit"

const defaultBaseURL = "https://api.typesafe.ai"

func validateBaseURL(raw string) (string, error) {
	return clientkit.ValidateBaseURL(raw, defaultBaseURL)
}
