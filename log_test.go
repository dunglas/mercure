package mercure

import (
	"bytes"
	"net/url"
	"testing"

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

var sink *MemorySink

func newTestLogger(t *testing.T) (*MemorySink, *zap.Logger) {
	t.Helper()

	if sink == nil {
		sink = &MemorySink{new(bytes.Buffer)}
		if err := zap.RegisterSink("memory", func(*url.URL) (zap.Sink, error) {
			return sink, nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	conf := zap.NewProductionConfig()
	conf.OutputPaths = []string{"memory://"}

	logger, err := conf.Build()
	if err != nil {
		t.Fatal(err)
	}

	return sink, logger
}
