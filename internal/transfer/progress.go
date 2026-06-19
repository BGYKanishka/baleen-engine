package transfer

import (
	"fmt"
	"sync/atomic"
	"time"
)

// wraps an io.Writer and tracks bytes written publishing progress events to the Hub
type progressWriter struct {
	writer    interface{ Write([]byte) (int, error) }
	total     int64
	sent      atomic.Int64
	image     string
	peer      string
	direction string
	lastTime  time.Time
	lastBytes int64
}

// publish events for the given image, peer, and direction
func newProgressWriter(w interface{ Write([]byte) (int, error) },
	total int64, image, peer, direction string) *progressWriter {
	return &progressWriter{
		writer:    w,
		total:     total,
		image:     image,
		peer:      peer,
		direction: direction,
		lastTime:  time.Now(),
	}
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if n > 0 {
		current := pw.sent.Add(int64(n))

		now := time.Now()
		elapsed := now.Sub(pw.lastTime).Seconds()

		// Publish at most ~10 times per second
		if elapsed >= 0.1 {
			bytesSinceLast := current - pw.lastBytes
			speedMBps := float64(bytesSinceLast) / elapsed / 1024 / 1024

			var progress float64
			if pw.total > 0 {
				progress = float64(current) / float64(pw.total) * 100
				if progress > 100 {
					progress = 100
				}
			}

			status := "transferring"
			if progress >= 100 {
				status = "completed"
			}

			GlobalHub.Publish(ProgressEvent{
				Direction: pw.direction,
				Image:     pw.image,
				Peer:      pw.peer,
				Progress:  progress,
				Speed:     fmt.Sprintf("%.2f MB/s", speedMBps),
				Status:    status,
			})

			pw.lastTime = now
			pw.lastBytes = current
		}
	}
	return n, err
}

// PublishStatus sends a one-off status event (e.g. "waiting", "failed")
func PublishStatus(image, peer, direction, status string) {
	GlobalHub.Publish(ProgressEvent{
		Direction: direction,
		Image:     image,
		Peer:      peer,
		Progress:  0,
		Speed:     "",
		Status:    status,
	})
}
