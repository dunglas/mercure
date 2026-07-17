//go:build deprecated_topic

package mercure

// allowsAlternateTopics reports whether v8 multi-topic publishes are accepted:
// the code is compiled in and the operator enabled compatibility mode.
func (h *Hub) allowsAlternateTopics() bool {
	return h.isBackwardCompatiblyEnabledWith(8)
}
