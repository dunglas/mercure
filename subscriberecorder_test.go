package mercure

import (
	"net/http/httptest"
	"os"
	"time"
)

// subscribeRecorder is a ResponseRecorder that also implements SetWriteDeadline
// / Flush — used by tests that drive SubscribeHandler against a synthetic
// response writer.
type subscribeRecorder struct {
	*httptest.ResponseRecorder

	writeDeadline time.Time
}

func newSubscribeRecorder() *subscribeRecorder {
	return &subscribeRecorder{ResponseRecorder: httptest.NewRecorder()}
}

// SetWriteDeadline mirrors net.Conn semantics: the latest call replaces the
// stored deadline, including a zero value which net.Conn documents as "no
// deadline". The earlier "only extend" version dropped the per-dispatch
// deadline that SubscribeHandler installs.
func (r *subscribeRecorder) SetWriteDeadline(deadline time.Time) error {
	r.writeDeadline = deadline

	return nil
}

func (r *subscribeRecorder) Write(buf []byte) (int, error) {
	if r.deadlineExceeded() {
		return 0, os.ErrDeadlineExceeded
	}

	return r.ResponseRecorder.Write(buf)
}

func (r *subscribeRecorder) FlushError() error {
	if r.deadlineExceeded() {
		return os.ErrDeadlineExceeded
	}

	r.Flush()

	return nil
}

func (*subscribeRecorder) Flush() {}

// deadlineExceeded reports whether the configured write deadline has
// passed. A zero deadline means "no deadline" (matching net.Conn) — under
// the previous logic time.Now().After(time.Time{}) was always true and any
// test that ran without setting a deadline would fail every Write.
func (r *subscribeRecorder) deadlineExceeded() bool {
	return !r.writeDeadline.IsZero() && time.Now().After(r.writeDeadline)
}
