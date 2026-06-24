//go:build deprecated_topic

package mercure

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
)

// Deprecated v8 URL shapes, registered only under the deprecated_topic build
// tag and WithProtocolVersionCompatibility(8).
const (
	subscriptionURL          = defaultHubURL + subscriptionsPath + "/{topic}/{subscriber}"
	subscriptionsForTopicURL = defaultHubURL + subscriptionsPath + "/{topic}"
)

// registerDeprecatedSubscriptionHandlers registers the v8
// /subscriptions/{topic}[/{subscriber}] routes when compatibility mode is
// enabled, and reports whether it took over the registration of the modern
// collection route.
func (h *Hub) registerDeprecatedSubscriptionHandlers(r *mux.Router) bool {
	if !h.isBackwardCompatiblyEnabledWith(8) {
		return false
	}

	r.HandleFunc(subscriptionsForMatchURL, h.SubscriptionsHandler).
		Methods(http.MethodGet).
		MatcherFunc(h.isKnownMatchType)

	r.HandleFunc(subscriptionURL, h.SubscriptionHandler).Methods(http.MethodGet)
	r.HandleFunc(subscriptionsForTopicURL, h.SubscriptionsHandler).Methods(http.MethodGet)

	return true
}

// subscriptionsForMatchPrefixLen is the length of the path prefix up to and
// including the trailing slash before the {matchType} segment, used to
// disambiguate the modern 2-segment collection route from the deprecated
// {topic}/{subscriber} route in isKnownMatchType.
const subscriptionsForMatchPrefixLen = len(defaultHubURL + subscriptionsPath + "/")

// isKnownMatchType is a mux.MatcherFunc that accepts requests whose
// {matchType} path segment is a matcher type defined by the protocol. Used
// to disambiguate the modern 2-segment collection route from the deprecated
// {topic}/{subscriber} route when backward compatibility is enabled. The
// path matcher has already accepted the overall shape, so we only need to
// peel off the first segment after /subscriptions/.
func (h *Hub) isKnownMatchType(r *http.Request, _ *mux.RouteMatch) bool {
	path := r.URL.EscapedPath()
	if len(path) <= subscriptionsForMatchPrefixLen {
		return false
	}

	segment, _, found := strings.Cut(path[subscriptionsForMatchPrefixLen:], "/")
	if !found {
		return false
	}

	mt, err := url.PathUnescape(segment)
	if err != nil {
		return false
	}

	t := MatcherType(mt)

	return t == MatcherTypeExact || t == MatcherTypeURLPattern
}
