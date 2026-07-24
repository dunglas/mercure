package mercure

// testUpdate builds an update from a v8-style topic list. Alternate topics
// are dropped in builds without the deprecated_topic tag; tests that rely on
// alternate-topic semantics are tagged deprecated_topic.
func testUpdate(u *Update, topics ...string) *Update {
	u.setTopics(topics)

	return u
}
