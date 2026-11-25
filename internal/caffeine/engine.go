package caffeine

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// State represents the high-level activity and sleep inhibition state.
type State string

const (
	// StateIdle indicates no active sessions, no cooldown, and no manual override. Host can sleep.
	StateIdle State = "IDLE"

	// StateActive indicates at least one agent session is actively executing. Host wake lock held.
	StateActive State = "ACTIVE"

	// StateCooldown indicates anti-flapping buffer countdown is draining before releasing lock.
	StateCooldown State = "COOLDOWN"

	// StateOverride indicates manual sleep inhibition lock is forced on.
	StateOverride State = "OVERRIDE"

	// StateWaitingApproval indicates session is awaiting human input with 120s grace period.
	StateWaitingApproval State = "WAITING_APPROVAL"
)

// Lease represents an active or waiting-approval agent execution lease.
type Lease struct {
	SessionID      string    `json:"session_id"`
	AgentName      string    `json:"agent_name,omitempty"`
	Task           string    `json:"task,omitempty"`
	PID            int       `json:"pid,omitempty"`
	State          State     `json:"state"`
	StartTime      time.Time `json:"start_time"`
	LastActiveTime time.Time `json:"last_active_time"`
	ApprovalExpiry time.Time `json:"approval_expiry,omitempty"`
	approvalTimer  *time.Timer
}

// LeaseInfo contains serializable snapshot data of an active lease.
type LeaseInfo struct {
	SessionID        string  `json:"session_id"`
	AgentName        string  `json:"agent_name,omitempty"`
	Task             string  `json:"task,omitempty"`
	PID              int     `json:"pid,omitempty"`
	State            State   `json:"state"`
	StartTime        string  `json:"start_time"`
	LastActiveTime   string  `json:"last_active_time"`
	ApprovalExpiry   string  `json:"approval_expiry,omitempty"`
	ApprovalGraceSec float64 `json:"approval_grace_remaining_seconds,omitempty"`
}

// JournalEntry records an audit entry for an activity state transition.
type JournalEntry struct {
	Timestamp time.Time `json:"timestamp"`
	FromState State     `json:"from_state"`
	ToState   State     `json:"to_state"`
	Reason    string    `json:"reason"`
	SessionID string    `json:"session_id,omitempty"`
}

// EngineConfig provides customizable parameters for Engine operation.
type EngineConfig struct {
	CooldownDuration      time.Duration
	ApprovalGraceDuration time.Duration
	BreakerCheckInterval  time.Duration
	BreakerConfig         CircuitBreakerConfig
	HistoryLimit          int
}

// DefaultEngineConfig returns production default configurations.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		CooldownDuration:      45 * time.Second,
		ApprovalGraceDuration: 120 * time.Second,
		BreakerCheckInterval:  5 * time.Second,
		BreakerConfig:         DefaultCircuitBreakerConfig(),
		HistoryLimit:          100,
	}
}

// EngineStatus returns a comprehensive snapshot of the caffeine engine.
type EngineStatus struct {
	State             State                `json:"state"`
	Inhibited         bool                 `json:"inhibited"`
	ActiveLeases      int                  `json:"active_leases"`
	Leases            []LeaseInfo          `json:"leases"`
	CooldownRemaining float64              `json:"cooldown_remaining"`
	OverrideRemaining float64              `json:"override_remaining"`
	OverrideActive    bool                 `json:"override_active"`
	Hardware          HardwareMetrics      `json:"hardware"`
	CircuitBreakers   CircuitBreakerStatus `json:"circuit_breakers"`
}

// Event represents an SSE or internal event broadcast to subscribers.
type Event struct {
	Type      string       `json:"type"` // "state_change", "heartbeat", "circuit_breaker"
	Timestamp time.Time    `json:"timestamp"`
	Status    EngineStatus `json:"status"`
}

// Engine coordinates multi-agent leases, anti-flapping cooldowns, and sleep inhibition locks.
type Engine struct {
	mu             sync.RWMutex
	config         EngineConfig
	inhibitor      Inhibitor
	hardwareReader HardwareReader

	releaseFunc   ReleaseFunc
	state         State
	leases        map[string]*Lease
	lockStartTime time.Time

	overrideActive  bool
	overrideEndTime time.Time
	overrideTimer   *time.Timer

	cooldownEndTime time.Time
	cooldownTimer   *time.Timer

	lastBreakerStatus CircuitBreakerStatus

	journal []JournalEntry

	subMu       sync.RWMutex
	subscribers map[chan Event]struct{}

	done   chan struct{}
	closed bool
}

// NewEngine creates and initializes a new Engine instance.
func NewEngine(cfg EngineConfig, inh Inhibitor, hr HardwareReader) *Engine {
	if cfg.CooldownDuration <= 0 {
		cfg.CooldownDuration = 45 * time.Second
	}
	if cfg.ApprovalGraceDuration <= 0 {
		cfg.ApprovalGraceDuration = 120 * time.Second
	}
	if cfg.BreakerCheckInterval <= 0 {
		cfg.BreakerCheckInterval = 5 * time.Second
	}
	if cfg.HistoryLimit <= 0 {
		cfg.HistoryLimit = 100
	}
	if inh == nil {
		inh = NewSystemInhibitor()
	}
	if hr == nil {
		hr = NewSystemHardwareReader()
	}

	e := &Engine{
		config:         cfg,
		inhibitor:      inh,
		hardwareReader: hr,
		state:          StateIdle,
		leases:         make(map[string]*Lease),
		journal:        make([]JournalEntry, 0, cfg.HistoryLimit),
		subscribers:    make(map[chan Event]struct{}),
		done:           make(chan struct{}),
	}

	go e.runBackgroundLoop()

	return e
}

// runBackgroundLoop periodically checks circuit breakers and broadcasts heartbeats.
func (e *Engine) runBackgroundLoop() {
	breakerTicker := time.NewTicker(e.config.BreakerCheckInterval)
	defer breakerTicker.Stop()

	heartbeatTicker := time.NewTicker(10 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-e.done:
			return
		case <-breakerTicker.C:
			e.mu.Lock()
			if !e.closed && e.releaseFunc != nil {
				e.checkBreakersLocked()
			}
			e.mu.Unlock()
		case <-heartbeatTicker.C:
			e.mu.RLock()
			if !e.closed {
				status := e.getStatusLocked()
				e.mu.RUnlock()
				e.broadcastEvent(Event{
					Type:      "heartbeat",
					Timestamp: time.Now(),
					Status:    status,
				})
			} else {
				e.mu.RUnlock()
			}
		}
	}
}

// Notify updates a session's state and triggers arbiter re-evaluation.
func (e *Engine) Notify(sessionID string, stateStr string, agent string, task string, pid int) (EngineStatus, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return e.getStatusLocked(), fmt.Errorf("engine is closed")
	}

	if sessionID == "" {
		return e.getStatusLocked(), fmt.Errorf("session_id cannot be empty")
	}

	normState := strings.ToLower(strings.TrimSpace(stateStr))
	now := time.Now()

	switch normState {
	case "active":
		// Stop any active global cooldown
		if e.cooldownTimer != nil {
			e.cooldownTimer.Stop()
			e.cooldownTimer = nil
			e.cooldownEndTime = time.Time{}
		}

		lease, exists := e.leases[sessionID]
		if exists {
			if lease.approvalTimer != nil {
				lease.approvalTimer.Stop()
				lease.approvalTimer = nil
			}
			lease.State = StateActive
			lease.LastActiveTime = now
			lease.ApprovalExpiry = time.Time{}
			if agent != "" {
				lease.AgentName = agent
			}
			if task != "" {
				lease.Task = task
			}
			if pid != 0 {
				lease.PID = pid
			}
		} else {
			e.leases[sessionID] = &Lease{
				SessionID:      sessionID,
				AgentName:      agent,
				Task:           task,
				PID:            pid,
				State:          StateActive,
				StartTime:      now,
				LastActiveTime: now,
			}
		}

		e.recomputeLocked(fmt.Sprintf("session '%s' active", sessionID), sessionID)

	case "waiting_approval", "waiting-approval", "approval":
		// Stop any active global cooldown
		if e.cooldownTimer != nil {
			e.cooldownTimer.Stop()
			e.cooldownTimer = nil
			e.cooldownEndTime = time.Time{}
		}

		expiry := now.Add(e.config.ApprovalGraceDuration)
		lease, exists := e.leases[sessionID]
		if exists {
			if lease.approvalTimer != nil {
				lease.approvalTimer.Stop()
			}
			lease.State = StateWaitingApproval
			lease.LastActiveTime = now
			lease.ApprovalExpiry = expiry
			if agent != "" {
				lease.AgentName = agent
			}
			if task != "" {
				lease.Task = task
			}
			if pid != 0 {
				lease.PID = pid
			}
		} else {
			lease = &Lease{
				SessionID:      sessionID,
				AgentName:      agent,
				Task:           task,
				PID:            pid,
				State:          StateWaitingApproval,
				StartTime:      now,
				LastActiveTime: now,
				ApprovalExpiry: expiry,
			}
			e.leases[sessionID] = lease
		}

		// Schedule 120-second grace timer for this lease
		sid := sessionID
		lease.approvalTimer = time.AfterFunc(e.config.ApprovalGraceDuration, func() {
			e.onApprovalExpired(sid)
		})

		e.recomputeLocked(fmt.Sprintf("session '%s' waiting approval (120s grace)", sessionID), sessionID)

	case "idle":
		lease, exists := e.leases[sessionID]
		if exists {
			if lease.approvalTimer != nil {
				lease.approvalTimer.Stop()
				lease.approvalTimer = nil
			}
			delete(e.leases, sessionID)
		}

		e.recomputeLocked(fmt.Sprintf("session '%s' idle", sessionID), sessionID)

	default:
		return e.getStatusLocked(), fmt.Errorf("invalid state '%s'; expected 'active', 'idle', or 'waiting_approval'", stateStr)
	}

	return e.getStatusLocked(), nil
}

// onApprovalExpired handles expiration of the 120s approval grace timer for a session.
func (e *Engine) onApprovalExpired(sessionID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}

	lease, exists := e.leases[sessionID]
	if exists && lease.State == StateWaitingApproval {
		lease.approvalTimer = nil
		delete(e.leases, sessionID)
		e.recomputeLocked(fmt.Sprintf("session '%s' approval grace expired after 120s", sessionID), sessionID)
	}
}

// SetOverride sets or clears manual sleep inhibition override.
func (e *Engine) SetOverride(enabled bool, durationStr string) (EngineStatus, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return e.getStatusLocked(), fmt.Errorf("engine is closed")
	}

	if e.overrideTimer != nil {
		e.overrideTimer.Stop()
		e.overrideTimer = nil
	}
	e.overrideEndTime = time.Time{}

	if enabled {
		if e.cooldownTimer != nil {
			e.cooldownTimer.Stop()
			e.cooldownTimer = nil
			e.cooldownEndTime = time.Time{}
		}

		dur, err := ParseDuration(durationStr)
		if err != nil {
			return e.getStatusLocked(), err
		}

		e.overrideActive = true
		if dur > 0 {
			e.overrideEndTime = time.Now().Add(dur)
			e.overrideTimer = time.AfterFunc(dur, func() {
				_, _ = e.SetOverride(false, "")
			})
		}

		e.recomputeLocked("manual override enabled", "")
	} else {
		e.overrideActive = false
		e.recomputeLocked("manual override disabled", "")
	}

	return e.getStatusLocked(), nil
}

// Cancel cancels active cooldown or manual override.
func (e *Engine) Cancel() (EngineStatus, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return e.getStatusLocked(), fmt.Errorf("engine is closed")
	}

	cancelledAny := false

	if e.overrideActive {
		e.overrideActive = false
		if e.overrideTimer != nil {
			e.overrideTimer.Stop()
			e.overrideTimer = nil
		}
		e.overrideEndTime = time.Time{}
		cancelledAny = true
	}

	if e.cooldownTimer != nil || e.state == StateCooldown {
		if e.cooldownTimer != nil {
			e.cooldownTimer.Stop()
			e.cooldownTimer = nil
		}
		e.cooldownEndTime = time.Time{}
		cancelledAny = true
	}

	if cancelledAny {
		// If no active or approval leases remain, force transition to IDLE without re-entering cooldown
		if !e.hasActiveOrWaitingLease() && !e.overrideActive {
			e.releaseLockLocked("cancelled")
			e.transitionStateLocked(StateIdle, "cancelled", "")
		} else {
			e.recomputeLocked("cancelled", "")
		}
	}

	return e.getStatusLocked(), nil
}

// onCooldownExpired handles expiration of the anti-flapping buffer.
func (e *Engine) onCooldownExpired() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}

	e.cooldownTimer = nil
	e.cooldownEndTime = time.Time{}

	// If no sessions became active during cooldown and override is not active, release lock
	if e.state == StateCooldown && !e.hasActiveOrWaitingLease() && !e.overrideActive {
		e.releaseLockLocked("cooldown buffer expired")
		e.transitionStateLocked(StateIdle, "cooldown buffer expired", "")
	}
}

// hasActiveOrWaitingLease reports if there are any active or approval-pending leases.
func (e *Engine) hasActiveOrWaitingLease() bool {
	for _, l := range e.leases {
		if l.State == StateActive || l.State == StateWaitingApproval {
			return true
		}
	}
	return false
}

// hasActiveLease reports if there are any leases currently in StateActive.
func (e *Engine) hasActiveLease() bool {
	for _, l := range e.leases {
		if l.State == StateActive {
			return true
		}
	}
	return false
}

// hasWaitingApprovalLease reports if there are any leases currently in StateWaitingApproval.
func (e *Engine) hasWaitingApprovalLease() bool {
	for _, l := range e.leases {
		if l.State == StateWaitingApproval {
			return true
		}
	}
	return false
}

// recomputeLocked determines the target state based on multi-agent disjunction ("Any Yes = YES").
func (e *Engine) recomputeLocked(reason string, sessionID string) {
	// 1. Hardware safety gate check
	if e.releaseFunc != nil {
		if tripped := e.checkBreakersLocked(); tripped {
			return
		}
	}

	// 2. Manual Override takes precedence
	if e.overrideActive {
		e.acquireLockLocked(reason)
		e.transitionStateLocked(StateOverride, reason, sessionID)
		return
	}

	// 3. Active lease check (Any active session = ACTIVE)
	if e.hasActiveLease() {
		e.acquireLockLocked(reason)
		e.transitionStateLocked(StateActive, reason, sessionID)
		return
	}

	// 4. Waiting approval lease check
	if e.hasWaitingApprovalLease() {
		e.acquireLockLocked(reason)
		e.transitionStateLocked(StateWaitingApproval, reason, sessionID)
		return
	}

	// 5. No active or waiting sessions remaining:
	// If coming from ACTIVE, WAITING_APPROVAL, or already in COOLDOWN:
	if e.state == StateActive || e.state == StateWaitingApproval || e.state == StateCooldown {
		if e.config.CooldownDuration > 0 {
			if e.cooldownTimer == nil {
				e.cooldownEndTime = time.Now().Add(e.config.CooldownDuration)
				e.cooldownTimer = time.AfterFunc(e.config.CooldownDuration, e.onCooldownExpired)
			}
			e.acquireLockLocked("anti-flapping cooldown buffer")
			e.transitionStateLocked(StateCooldown, reason, sessionID)
			return
		}
	}

	// 6. Otherwise cleanly transition to IDLE and release lock
	e.releaseLockLocked(reason)
	e.transitionStateLocked(StateIdle, reason, sessionID)
}

// checkBreakersLocked evaluates hardware safety rules and drops the lock if tripped.
func (e *Engine) checkBreakersLocked() bool {
	metrics, err := e.hardwareReader.ReadMetrics()
	if err != nil {
		return false
	}

	status := EvaluateCircuitBreakers(metrics, e.lockStartTime, e.config.BreakerConfig)
	e.lastBreakerStatus = status

	if status.Tripped {
		e.releaseLockLocked(status.Reason)
		e.overrideActive = false
		if e.overrideTimer != nil {
			e.overrideTimer.Stop()
			e.overrideTimer = nil
		}
		e.overrideEndTime = time.Time{}
		if e.cooldownTimer != nil {
			e.cooldownTimer.Stop()
			e.cooldownTimer = nil
		}
		e.cooldownEndTime = time.Time{}

		e.transitionStateLocked(StateIdle, "circuit breaker tripped: "+status.Reason, "")

		statusSnap := e.getStatusLocked()
		go e.broadcastEvent(Event{
			Type:      "circuit_breaker",
			Timestamp: time.Now(),
			Status:    statusSnap,
		})
		return true
	}

	return false
}

// acquireLockLocked ensures the system sleep inhibitor is held.
func (e *Engine) acquireLockLocked(reason string) {
	if e.releaseFunc != nil {
		return
	}

	rel, err := e.inhibitor.Inhibit(reason)
	if err == nil {
		e.releaseFunc = rel
		e.lockStartTime = time.Now()
	}
}

// releaseLockLocked releases the system sleep inhibitor if held.
func (e *Engine) releaseLockLocked(reason string) {
	if e.releaseFunc != nil {
		_ = e.releaseFunc()
		e.releaseFunc = nil
		e.lockStartTime = time.Time{}
	}
}

// transitionStateLocked updates state, appends to journal, and broadcasts change if state differed.
func (e *Engine) transitionStateLocked(newState State, reason string, sessionID string) {
	oldState := e.state
	if oldState != newState {
		e.state = newState
		entry := JournalEntry{
			Timestamp: time.Now(),
			FromState: oldState,
			ToState:   newState,
			Reason:    reason,
			SessionID: sessionID,
		}
		e.journal = append(e.journal, entry)
		if len(e.journal) > e.config.HistoryLimit {
			e.journal = e.journal[len(e.journal)-e.config.HistoryLimit:]
		}

		status := e.getStatusLocked()
		go e.broadcastEvent(Event{
			Type:      "state_change",
			Timestamp: time.Now(),
			Status:    status,
		})
	}
}

// GetStatus returns the current status snapshot.
func (e *Engine) GetStatus() EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.getStatusLocked()
}

// getStatusLocked builds the status struct assuming mutex is already held.
func (e *Engine) getStatusLocked() EngineStatus {
	now := time.Now()

	cooldownRem := 0.0
	if e.state == StateCooldown && !e.cooldownEndTime.IsZero() && e.cooldownEndTime.After(now) {
		cooldownRem = e.cooldownEndTime.Sub(now).Seconds()
	}

	overrideRem := 0.0
	if e.overrideActive {
		if e.overrideEndTime.IsZero() {
			overrideRem = -1.0 // indefinite
		} else if e.overrideEndTime.After(now) {
			overrideRem = e.overrideEndTime.Sub(now).Seconds()
		}
	}

	leasesList := make([]LeaseInfo, 0, len(e.leases))
	activeCount := 0
	for _, l := range e.leases {
		if l.State == StateActive {
			activeCount++
		}
		graceRem := 0.0
		if !l.ApprovalExpiry.IsZero() && l.ApprovalExpiry.After(now) {
			graceRem = l.ApprovalExpiry.Sub(now).Seconds()
		}
		leasesList = append(leasesList, LeaseInfo{
			SessionID:        l.SessionID,
			AgentName:        l.AgentName,
			Task:             l.Task,
			PID:              l.PID,
			State:            l.State,
			StartTime:        l.StartTime.Format(time.RFC3339),
			LastActiveTime:   l.LastActiveTime.Format(time.RFC3339),
			ApprovalExpiry:   l.ApprovalExpiry.Format(time.RFC3339),
			ApprovalGraceSec: graceRem,
		})
	}

	hwMetrics, _ := e.hardwareReader.ReadMetrics()
	cbStatus := EvaluateCircuitBreakers(hwMetrics, e.lockStartTime, e.config.BreakerConfig)

	return EngineStatus{
		State:             e.state,
		Inhibited:         e.releaseFunc != nil,
		ActiveLeases:      activeCount,
		Leases:            leasesList,
		CooldownRemaining: cooldownRem,
		OverrideRemaining: overrideRem,
		OverrideActive:    e.overrideActive,
		Hardware:          hwMetrics,
		CircuitBreakers:   cbStatus,
	}
}

// GetHistory returns up to limit recent journal entries.
func (e *Engine) GetHistory(limit int) []JournalEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.journal) {
		limit = len(e.journal)
	}

	start := len(e.journal) - limit
	result := make([]JournalEntry, limit)
	copy(result, e.journal[start:])
	return result
}

// Subscribe registers a new subscriber channel for events.
func (e *Engine) Subscribe() chan Event {
	e.subMu.Lock()
	defer e.subMu.Unlock()

	ch := make(chan Event, 32)
	e.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes and closes a subscriber channel.
func (e *Engine) Unsubscribe(ch chan Event) {
	e.subMu.Lock()
	defer e.subMu.Unlock()

	if _, ok := e.subscribers[ch]; ok {
		delete(e.subscribers, ch)
		close(ch)
	}
}

// broadcastEvent non-blockingly delivers an event to all subscribers.
func (e *Engine) broadcastEvent(event Event) {
	e.subMu.RLock()
	defer e.subMu.RUnlock()

	for ch := range e.subscribers {
		select {
		case ch <- event:
		default:
			// Non-blocking drop if consumer buffer is full
		}
	}
}

// Close shuts down the engine, clears timers, and releases system locks.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true
	close(e.done)

	if e.cooldownTimer != nil {
		e.cooldownTimer.Stop()
		e.cooldownTimer = nil
	}
	if e.overrideTimer != nil {
		e.overrideTimer.Stop()
		e.overrideTimer = nil
	}
	for _, l := range e.leases {
		if l.approvalTimer != nil {
			l.approvalTimer.Stop()
			l.approvalTimer = nil
		}
	}

	e.releaseLockLocked("engine closed")

	e.subMu.Lock()
	for ch := range e.subscribers {
		close(ch)
	}
	e.subscribers = make(map[chan Event]struct{})
	e.subMu.Unlock()

	return nil
}

// ParseDuration parses duration strings such as "1h", "30m", "1h30m", "indefinite", "forever".
func ParseDuration(s string) (time.Duration, error) {
	clean := strings.ToLower(strings.TrimSpace(s))
	if clean == "" || clean == "indefinite" || clean == "forever" || clean == "0" || clean == "none" || clean == "infinite" {
		return 0, nil
	}

	// Normalize human words
	clean = strings.ReplaceAll(clean, "hours", "h")
	clean = strings.ReplaceAll(clean, "hour", "h")
	clean = strings.ReplaceAll(clean, "minutes", "m")
	clean = strings.ReplaceAll(clean, "minute", "m")
	clean = strings.ReplaceAll(clean, "mins", "m")
	clean = strings.ReplaceAll(clean, "min", "m")
	clean = strings.ReplaceAll(clean, "seconds", "s")
	clean = strings.ReplaceAll(clean, "second", "s")
	clean = strings.ReplaceAll(clean, "secs", "s")
	clean = strings.ReplaceAll(clean, "sec", "s")
	clean = strings.ReplaceAll(clean, " ", "")

	dur, err := time.ParseDuration(clean)
	if err != nil {
		return 0, fmt.Errorf("invalid duration '%s': %w", s, err)
	}
	if dur <= 0 {
		return 0, fmt.Errorf("duration must be greater than zero: '%s'", s)
	}
	return dur, nil
}
