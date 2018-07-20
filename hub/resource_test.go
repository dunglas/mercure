package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewResource(t *testing.T) {
	r := NewResource("http://example.com", "foo\nbar", map[string]bool{"baz": true, "bat": true})
	assert.Equal(t, "http://example.com", r.IRI)
	assert.Equal(t, "data: foo\ndata: bar\n\n", r.Data)
	assert.Equal(t, map[string]bool{"baz": true, "bat": true}, r.Targets)
}
