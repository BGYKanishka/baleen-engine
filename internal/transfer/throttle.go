package transfer

import (
	"context"
	"io"

	"golang.org/x/time/rate"
)

// wraps an io.Reader and limits the read rate to bytesPerSec.
type ThrottledReader struct {
	r       io.Reader
	limiter *rate.Limiter
	ctx     context.Context
}

// creates a new ThrottledReader.
func NewThrottledReader(r io.Reader, bytesPerSec int) *ThrottledReader {
	if bytesPerSec <= 0 {
		return &ThrottledReader{r: r, limiter: nil, ctx: context.Background()}
	}
	burst := 32 * 1024
	if bytesPerSec < burst {
		burst = bytesPerSec
	}
	limiter := rate.NewLimiter(rate.Limit(bytesPerSec), burst)
	return &ThrottledReader{
		r:       r,
		limiter: limiter,
		ctx:     context.Background(),
	}
}

// Read reads from the underlying reader and blocks until the rate limiter allows it.
func (t *ThrottledReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 && t.limiter != nil {
		// WaitN blocks until n tokens are available or the context is cancelled.
		if waitErr := t.limiter.WaitN(t.ctx, n); waitErr != nil {
			return n, waitErr
		}
	}
	return n, err
}
