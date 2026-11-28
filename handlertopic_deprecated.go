//go:build deprecated_topic

package mercure

import "net/http"

// subscriptionsForTopicURL is the deprecated v8 URL shape, registered only
// under the deprecated_topic build tag and WithProtocolVersionCompatibility(8).
const subscriptionsForTopicURL = defaultHubURL + subscriptionsPath + "/{topic}"

// registerDeprecatedSubscriptionHandlers registers the v8
// /subscriptions/{topic}[/{subscriber}] routes when compatibility mode is
// enabled, and reports whether it took over the registration of the modern
// collection route.
func (h *Hub) registerDeprecatedSubscriptionHandlers(router *http.ServeMux) bool {
	if !h.isBackwardCompatiblyEnabledWith(8) {
		return false
	}

	router.HandleFunc("GET "+subscriptionsForMatchURL, h.subscriptionsForMatchOrTopicHandler)
	router.HandleFunc("GET "+subscriptionsForTopicURL, h.SubscriptionsHandler)

	return true
}

// subscriptionsForMatchOrTopicHandler serves both /{match_type}/{match} and
// the v8 /{topic}/{subscriber}, which ServeMux cannot register side by side:
// a known matcher type selects the modern route.
func (h *Hub) subscriptionsForMatchOrTopicHandler(w http.ResponseWriter, r *http.Request) {
	if knownMatcherType(MatcherType(r.PathValue(paramMatchType))) {
		h.SubscriptionsHandler(w, r)

		return
	}

	r.SetPathValue(paramTopic, r.PathValue(paramMatchType))
	r.SetPathValue(paramSubscriber, r.PathValue(paramMatch))
	r.SetPathValue(paramMatchType, "")
	r.SetPathValue(paramMatch, "")

	h.SubscriptionHandler(w, r)
}
