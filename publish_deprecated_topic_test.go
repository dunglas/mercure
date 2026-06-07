//go:build deprecated_topic

package mercure

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUpdateValidateTooManyTopics needs the deprecated_topic tag: only v8
// updates can carry alternate topics, so the cap is unreachable otherwise.
func TestUpdateValidateTooManyTopics(t *testing.T) {
	t.Parallel()

	topics := make([]string, maxPublishTopics+1)
	for i := range topics {
		topics[i] = "https://example.com/books/1"
	}

	err := testUpdate(&Update{}, topics...).Validate()
	assert.True(t, errors.Is(err, ErrTooManyTopics), "got %v, want %v", err, ErrTooManyTopics)
}
