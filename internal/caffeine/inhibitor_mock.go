package caffeine

import (
	"errors"
	"sync"
)

// MockInhibitor is an in-memory mock inhibitor for testing.
type MockInhibitor struct {
	mu           sync.Mutex
	active       bool
	reason       string
	inhibitCalls int
	releaseCalls int
	failInhibit  bool
	failRelease  bool
	available    bool
}

// NewMockInhibitor creates a new MockInhibitor.
func NewMockInhibitor() *MockInhibitor {
	return &MockInhibitor{
		available: true,
	}
}

// Inhibit records an inhibit call and returns a ReleaseFunc.
func (m *MockInhibitor) Inhibit(reason string) (ReleaseFunc, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.inhibitCalls++
	if m.failInhibit {
		return nil, errors.New("mock inhibit failure")
	}

	m.active = true
	m.reason = reason

	var once sync.Once
	return func() error {
		var err error
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.releaseCalls++
			if m.failRelease {
				err = errors.New("mock release failure")
				return
			}
			m.active = false
		})
		return err
	}, nil
}

// Name returns the identifier of the mock inhibitor.
func (m *MockInhibitor) Name() string {
	return "mock"
}

// IsAvailable reports whether the mock inhibitor is available.
func (m *MockInhibitor) IsAvailable() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.available
}

// IsActive reports whether the mock inhibitor is currently holding a lock.
func (m *MockInhibitor) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// Reason returns the reason provided in the last Inhibit call.
func (m *MockInhibitor) Reason() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reason
}

// InhibitCalls returns the total count of Inhibit invocations.
func (m *MockInhibitor) InhibitCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inhibitCalls
}

// ReleaseCalls returns the total count of ReleaseFunc invocations.
func (m *MockInhibitor) ReleaseCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.releaseCalls
}

// SetFailInhibit configures whether future Inhibit calls should return an error.
func (m *MockInhibitor) SetFailInhibit(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failInhibit = fail
}

// SetFailRelease configures whether future ReleaseFunc calls should return an error.
func (m *MockInhibitor) SetFailRelease(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failRelease = fail
}

// SetAvailable sets the availability flag.
func (m *MockInhibitor) SetAvailable(available bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.available = available
}
