package transfer

import (
	"sync"
)

// holds callbacks for the pre-streaming phase.
type PendingTransfer struct {
	// CancelConn closes the live data connection, causing negotiate() to fail.
	CancelConn func()
	// SendControl opens a new connection to the receiver and sends a control message.
	SendControl func(action, initiator string)
}

// holds the channel for an in-flight receiver approval.
type PendingApproval struct {
	RespChan chan bool
	Peer     string
}

type Manager struct {
	mu               sync.RWMutex
	transfers        map[string]*progressWriter
	pendingApprovals map[string]*PendingApproval
	pendingConns     map[string]*PendingTransfer
}

var GlobalManager = &Manager{
	transfers:        make(map[string]*progressWriter),
	pendingApprovals: make(map[string]*PendingApproval),
	pendingConns:     make(map[string]*PendingTransfer),
}

//Active transfers

func (m *Manager) Register(pw *progressWriter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transfers[hubKey(pw.image, pw.peer)] = pw
}

func (m *Manager) Unregister(pw *progressWriter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.transfers, hubKey(pw.image, pw.peer))
}

func (m *Manager) Get(image, peer string) (*progressWriter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pw, ok := m.transfers[hubKey(image, peer)]
	return pw, ok
}

//  Pending approvals (receiver side)

// records the channel while receiveAndApprove blocks.
func (m *Manager) RegisterApproval(image, peer string, respChan chan bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingApprovals[image] = &PendingApproval{RespChan: respChan, Peer: peer}
}

// removes the record after approval is resolved.
func (m *Manager) UnregisterApproval(image string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pendingApprovals, image)
}

// rejects a pending approval. Returns the peer and if found.
func (m *Manager) CancelApproval(image string) (peer string, ok bool) {
	m.mu.Lock()
	pa, found := m.pendingApprovals[image]
	if found {
		delete(m.pendingApprovals, image)
	}
	m.mu.Unlock()

	if found {
		// Non-blocking send: safe if already responded.
		select {
		case pa.RespChan <- false:
		default:
		}
		return pa.Peer, true
	}
	return "", false
}

// Pending connections (sender side, pre-streaming)

// records a cancellable pre-streaming connection.
func (m *Manager) RegisterPendingConn(image, peer string, pt *PendingTransfer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingConns[hubKey(image, peer)] = pt
}

// removes the record.
func (m *Manager) UnregisterPendingConn(image, peer string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pendingConns, hubKey(image, peer))
}

// returns the pending transfer if it exists.
func (m *Manager) GetPendingConn(image, peer string) (*PendingTransfer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pt, ok := m.pendingConns[hubKey(image, peer)]
	return pt, ok
}
