//go:build deprecated_topic

package mercure

// deprecatedTopics carries the v8 alternate topics, removed from the modern
// protocol (an update has exactly one topic).
type deprecatedTopics struct {
	// Topics holds alternate topic IRIs in addition to Update.Topic.
	//
	// Deprecated: alternate topics were removed from the protocol; this field
	// only exists in builds with the deprecated_topic tag.
	Topics []string
}

// topics returns the canonical topic followed by the v8 alternate topics.
// Updates built by legacy code that only sets Topics keep working: the first
// element acts as the canonical topic.
func (u *Update) topics() []string {
	if u.Topic == "" && len(u.Topics) > 0 {
		return u.Topics
	}

	if len(u.Topics) == 0 {
		return []string{u.Topic}
	}

	return append([]string{u.Topic}, u.Topics...)
}

// setTopics assigns the canonical topic and the v8 alternates from a v8-style
// topic list.
func (u *Update) setTopics(topics []string) {
	u.Topic, u.Topics = "", nil

	if len(topics) == 0 {
		return
	}

	u.Topic = topics[0]
	if len(topics) > 1 {
		u.Topics = topics[1:]
	}
}
