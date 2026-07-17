//go:build !deprecated_topic

package mercure

// deprecatedTopics is empty without the deprecated_topic build tag: an update
// has exactly one topic.
type deprecatedTopics struct{} //nolint:unused // embedded in Update for build-mode symmetry, like deprecatedHub in Hub

func (u *Update) topics() []string {
	return []string{u.Topic}
}

// setTopics keeps only the canonical topic; v8 alternate topics are not
// supported in this build mode.
func (u *Update) setTopics(topics []string) {
	u.Topic = ""
	if len(topics) > 0 {
		u.Topic = topics[0]
	}
}
