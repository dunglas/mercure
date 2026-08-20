package mercure

import (
	"encoding/json"
	"net/url"
)

// Media Type for Events Query in JSON
const eventsJSONMediaType = "application/events+json"

// parseEventsJSON reads a JSON Events Query into the values a form-encoded
// subscription would have carried, so that one implementation validates both.
// A property that may be absent is decoded into a pointer and becomes a key
// only when it was supplied, which is the presence a form body expresses by
// carrying the parameter at all.
//
// The "state" property is ignored: Mercure hubs serve notifications only and
// a hub has no representation to deliver alongside them.
func parseEventsJSON(body []byte) (url.Values, error) {
	var subscription eventsQuerySubscription

	// The decoder's own errors name the offending property and its offset, so
	// they are returned as they are — parseSubscriptionBody names the media
	// type once, for every parser.
	if err := json.Unmarshal(body, &subscription); err != nil {
		return nil, err
	}

	values := subscription.URL.values()

	if subscription.Events != nil {
		values["events"] = []string{""}
	}

	// An empty cursor is still a cursor: the protocol answers with a
	// Mercure-Last-Event-Id field whenever one was supplied at all, which is
	// why presence rather than value decides.
	if subscription.LastEventID != nil {
		values["last_event_id"] = []string{*subscription.LastEventID}
	}

	return values, nil
}

// eventsQuerySubscription is the JSON realization of the subscription data
// model. A pointer distinguishes a property that was absent or null from one
// that was supplied, which is what the "events" property turns on.
//
// The "state" property has no field: it asks for a representation, of which a
// hub has none, and properties without a field are ignored.
type eventsQuerySubscription struct {
	URL         *eventsQueryURL    `json:"url"`
	Events      *eventsQueryEvents `json:"events"`
	LastEventID *string            `json:"last_event_id"`
}

// eventsQueryEvents describes the notifications asked for. Its presence
// is the interest in a stream; its sub-properties will describe the form
// they take.
type eventsQueryEvents struct{}

// eventsQueryURL is the "url" property of Mercure: an object naming a topic
// matcher type per key, or an array of topics as shorthand for the default
// matcher type, so that
//
//	"url": ["https://example.com/books/1"]
//
// means the same as
//
//	"url": {"match": ["https://example.com/books/1"]}
//
// Matcher parameter names are kept as they arrive, including ones outside
// the matcher namespace. Which names are meaningful, which are reserved and
// which are rejected is decided once, where the matchers are parsed, so the
// JSON and the query component cannot drift apart.
type eventsQueryURL map[string]eventsQueryTopics

func (u *eventsQueryURL) UnmarshalJSON(data []byte) error {
	if data[0] == '[' {
		var topics []string
		if err := json.Unmarshal(data, &topics); err != nil {
			return err //nolint:wrapcheck
		}

		*u = eventsQueryURL{paramMatch: topics}

		return nil
	}

	var named map[string]eventsQueryTopics
	if err := json.Unmarshal(data, &named); err != nil {
		return err //nolint:wrapcheck
	}

	*u = named

	return nil
}

// values converts the property into the name/value pairs the subscription
// URL's query component would have carried. A nil property carried none.
func (u *eventsQueryURL) values() url.Values {
	if u == nil {
		return url.Values{}
	}

	values := make(url.Values, len(*u))
	for name, topics := range *u {
		values[name] = topics
	}

	return values
}

// eventsQueryTopics is one topic or an array of them.
type eventsQueryTopics []string

func (t *eventsQueryTopics) UnmarshalJSON(data []byte) error {
	// A null value reaches an Unmarshaler too, and would otherwise decode
	// down the scalar branch as a single empty topic.
	if string(data) == "null" {
		return nil
	}

	if data[0] == '[' {
		return json.Unmarshal(data, (*[]string)(t)) //nolint:wrapcheck
	}

	var one string
	if err := json.Unmarshal(data, &one); err != nil {
		return err //nolint:wrapcheck
	}

	*t = eventsQueryTopics{one}

	return nil
}
