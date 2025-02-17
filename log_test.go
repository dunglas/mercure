package mercure

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"go.uber.org/zap"
)

// MemorySink implements zap.Sink by writing all messages to a buffer.
type MemorySink struct {
	*bytes.Buffer
}

// Implement Close and Sync as no-ops to satisfy the interface. The Write
// method is provided by the embedded buffer.

func (s *MemorySink) Close() error { return nil }
func (s *MemorySink) Sync() error  { return nil }

func newTestLogger(t *testing.T) (*MemorySink, *zap.Logger) {
	t.Helper()

	sink := &MemorySink{new(bytes.Buffer)}
	require.NoError(t, zap.RegisterSink(t.Name(), func(*url.URL) (zap.Sink, error) {
		return sink, nil
	}))

	conf := zap.NewProductionConfig()
	conf.OutputPaths = []string{t.Name() + "://"}

	logger, err := conf.Build()
	require.NoError(t, err)

	return sink, logger
}
