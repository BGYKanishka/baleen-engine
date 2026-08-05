package transfer

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// progressWriter wraps an io.Writer, tracking bytes and publishing hub events.
type progressWriter struct {
	writer    interface{ Write([]byte) (int, error) }
	total     int64
	sent      atomic.Int64
	image     string
	peer      string
	direction string // "push" | "pull"
	lastTime  time.Time
	lastBytes int64

	// buffered (size 1) so signals aren't lost when Write() isn't in the select
	resumeCh chan struct{}
	cancelCh chan struct{}

	paused   atomic.Bool
	canceled atomic.Bool
	mu       sync.Mutex
	// pausedBy: "sender" | "receiver" | "" — only that side may resume
	pausedBy string

	// ControlConn: pull-side back-channel; single bytes P/R/C flow back to sender
	ControlConn interface{ Write([]byte) (int, error) }

	// notifyPeer: push-side only; opens a fresh connection to notify the receiver
	notifyPeer func(action, initiator string)
}

func newProgressWriter(w interface{ Write([]byte) (int, error) },
	total int64, image, peer, direction string) *progressWriter {
	pw := &progressWriter{
		writer:    w,
		total:     total,
		image:     image,
		peer:      peer,
		direction: direction,
		lastTime:  time.Now(),
		resumeCh:  make(chan struct{}, 1),
		cancelCh:  make(chan struct{}, 1),
	}
	GlobalManager.Register(pw)
	return pw
}

func (pw *progressWriter) Direction() string { return pw.direction }

func (pw *progressWriter) Write(p []byte) (int, error) {
	if pw.canceled.Load() {
		return 0, fmt.Errorf("transfer cancelled")
	}

	if pw.paused.Load() {
		pw.publishStatus("paused", pw.currentProgress(), "")
		select {
		case <-pw.resumeCh:
		case <-pw.cancelCh:
			return 0, fmt.Errorf("transfer cancelled")
		}
	}

	n, err := pw.writer.Write(p)
	if n > 0 {
		current := pw.sent.Add(int64(n))
		now := time.Now()
		elapsed := now.Sub(pw.lastTime).Seconds()
		if elapsed >= 0.1 {
			bytesSinceLast := current - pw.lastBytes
			speedMBps := float64(bytesSinceLast) / elapsed / 1024 / 1024
			progress := pw.progressFromBytes(current)
			status := "transferring"
			if progress >= 100 {
				status = "completed"
			}
			pw.publishStatus(status, progress, fmt.Sprintf("%.2f MB/s", speedMBps))
			pw.lastTime = now
			pw.lastBytes = current
		}
	}
	return n, err
}

func (pw *progressWriter) publishStatus(status string, progress float64, speed string) {
	GlobalHub.Publish(ProgressEvent{
		Direction: pw.direction,
		Image:     pw.image,
		Peer:      pw.peer,
		Progress:  progress,
		Speed:     speed,
		Status:    status,
	})
}

func (pw *progressWriter) currentProgress() float64 {
	return pw.progressFromBytes(pw.sent.Load())
}

func (pw *progressWriter) progressFromBytes(bytes int64) float64 {
	if pw.total <= 0 {
		return 0
	}
	p := float64(bytes) / float64(pw.total) * 100
	if p > 100 {
		return 100
	}
	return p
}

// returns true when the action was initiated by this side.
func (pw *progressWriter) isLocalInitiator(initiator string) bool {
	return (pw.direction == "push" && initiator == "sender") ||
		(pw.direction == "pull" && initiator == "receiver")
}

// notifies the remote side via ControlConn (pull) or notifyPeer (push).
func (pw *progressWriter) signalPeer(b byte, action, initiator string) {
	if pw.ControlConn != nil {
		pw.ControlConn.Write([]byte{b}) //nolint:errcheck
	} else if pw.notifyPeer != nil {
		go pw.notifyPeer(action, initiator)
	}
}

// Pause pauses the transfer. Only the initiating side may later resume it.
func (pw *progressWriter) Pause(initiator string) {
	if pw.paused.Load() || pw.canceled.Load() {
		return
	}
	pw.paused.Store(true)
	pw.mu.Lock()
	pw.pausedBy = initiator
	pw.mu.Unlock()
	pw.publishStatus("paused", pw.currentProgress(), "")
	if pw.isLocalInitiator(initiator) {
		pw.signalPeer('P', "pause", initiator)
	}
}

// Resume resumes the transfer. Returns an error if the caller didn't pause it.
func (pw *progressWriter) Resume(requester string) error {
	if pw.canceled.Load() {
		return fmt.Errorf("transfer was cancelled")
	}
	pw.mu.Lock()
	if pw.pausedBy != "" && pw.pausedBy != requester {
		pw.mu.Unlock()
		return fmt.Errorf("only the %s can resume this paused transfer", pw.pausedBy)
	}

	prevPausedBy := pw.pausedBy
	pw.pausedBy = ""
	pw.mu.Unlock()

	if pw.paused.Load() {
		pw.paused.Store(false)
		select {
		case pw.resumeCh <- struct{}{}:
		default:
		}
	}
	pw.publishStatus("transferring", pw.currentProgress(), "")
	if pw.isLocalInitiator(prevPausedBy) {
		pw.signalPeer('R', "resume", requester)
	}
	return nil
}

// Cancel cancels the transfer from either side.
func (pw *progressWriter) Cancel() {
	if pw.canceled.Load() {
		return
	}
	pw.canceled.Store(true)
	pw.publishStatus("cancelled", pw.currentProgress(), "")
	pw.signalPeer('C', "cancel", "")
	select {
	case pw.cancelCh <- struct{}{}:
	default:
	}
	select {
	case pw.resumeCh <- struct{}{}:
	default:
	}
}

// updates UI status to "paused" without blocking io.Copy (remote paused).
func (pw *progressWriter) NotifyPausedBy(initiator string) {
	pw.mu.Lock()
	pw.pausedBy = initiator
	pw.mu.Unlock()
	pw.publishStatus("paused", pw.currentProgress(), "")
}

// clears the paused state on a status-only basis (remote resumed).
func (pw *progressWriter) NotifyResumed() {
	pw.mu.Lock()
	pw.pausedBy = ""
	pw.mu.Unlock()
	pw.publishStatus("transferring", pw.currentProgress(), "")
}

func (pw *progressWriter) Cleanup() {
	GlobalManager.Unregister(pw)
}

// sends a one-off status event (e.g. "waiting", "failed").
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
