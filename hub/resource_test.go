package hub

import (
	"testing"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewResource(t *testing.T) {
	r := NewResource("foo", "http://example.com", "foo\nbar", map[string]struct{}{"baz": struct{}{}, "bat": struct{}{}})
	assert.Equal(t, "foo", r.RevID)
	assert.Equal(t, "http://example.com", r.IRI)
	assert.Equal(t, "data: foo\ndata: bar\n\n", r.Data)
	assert.Equal(t, map[string]struct{}{"baz": struct{}{}, "bat": struct{}{}}, r.Targets)
}

func TestGenerateID(t *testing.T) {
	r := NewResource("", "http://example.com", "foo\nbar", map[string]struct{}{"baz": struct{}{}, "bat": struct{}{}})

	_, err := uuid.FromString(r.RevID)
	assert.Nil(t, err)
}
