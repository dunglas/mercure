//go:build !deprecated_topic

package mercure

// allowsAlternateTopics is the stub compiled without the deprecated_topic
// build tag: an update has exactly one topic.
func (h *Hub) allowsAlternateTopics() bool {
	return false
}
