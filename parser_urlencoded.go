package mercure

import (
	"net/url"
)

// Media type of a subscription expressed as form-encoded parameters, the
// shape a query component carries.
const urlEncodedMediaType = "application/x-www-form-urlencoded"

// parseURLEncoded parses a form-encoded (application/x-www-form-urlencoded)
// representation.
func parseURLEncoded(body []byte) (url.Values, error) {
	return url.ParseQuery(string(body))
}
