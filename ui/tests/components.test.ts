import { describe, it, expect, vi, beforeEach } from 'vitest';

// ---------------------------------------------------------------------------
// Hermetic DOM Environment Mock
// ---------------------------------------------------------------------------

class MockClassList {
  private el: MockElement;
  public classes: Set<string> = new Set();

  constructor(el: MockElement) {
    this.el = el;
  }

  add(...classes: string[]): void {
    for (const c of classes) {
      if (c && typeof c === 'string') {
        c.split(/\s+/).filter(Boolean).forEach((cls) => this.classes.add(cls));
      }
    }
    this._sync();
  }

  remove(...classes: string[]): void {
    for (const c of classes) {
      if (c && typeof c === 'string') {
        c.split(/\s+/).filter(Boolean).forEach((cls) => this.classes.delete(cls));
      }
    }
    this._sync();
  }

  contains(cls: string): boolean {
    return this.classes.has(cls);
  }

  toggle(cls: string, force?: boolean): boolean {
    if (force === true) {
      this.add(cls);
      return true;
    }
    if (force === false) {
      this.remove(cls);
      return false;
    }
    if (this.contains(cls)) {
      this.remove(cls);
      return false;
    }
    this.add(cls);
    return true;
  }

  _sync(): void {
    this.el._className = Array.from(this.classes).join(' ');
  }

  get length(): number {
    return this.classes.size;
  }
}

class MockElement {
  public tagName: string;
  public _className = '';
  public classList: MockClassList;
  public attributes: Map<string, string> = new Map();
  public children: MockElement[] = [];
  public parentElement: MockElement | null = null;
  public listeners: Map<string, Array<(e: any) => void>> = new Map();
  public style: Record<string, string> = {};
  public id = '';
  public _textContent = '';
  public disabled = false;

  constructor(tagName = 'DIV') {
    this.tagName = tagName.toUpperCase();
    this.classList = new MockClassList(this);
  }

  get className(): string {
    return this._className;
  }

  set className(val: string) {
    this._className = val || '';
    this.classList.classes.clear();
    if (val) {
      val.split(/\s+/).filter(Boolean).forEach((c) => this.classList.classes.add(c));
    }
  }

  get textContent(): string {
    if (this.children.length === 0) return this._textContent;
    return this.children.map((c) => c.textContent).join('');
  }

  set textContent(val: string) {
    this._textContent = String(val);
    this.children = [];
  }

  appendChild<T extends MockElement>(child: T): T {
    child.parentElement = this;
    this.children.push(child);
    return child;
  }

  removeChild<T extends MockElement>(child: T): T {
    const idx = this.children.indexOf(child);
    if (idx !== -1) {
      this.children.splice(idx, 1);
      child.parentElement = null;
    }
    return child;
  }

  get firstChild(): MockElement | null {
    return this.children[0] || null;
  }

  setAttribute(name: string, value: string): void {
    this.attributes.set(name, String(value));
    if (name === 'id') this.id = String(value);
    if (name === 'class') this.className = String(value);
    if (name === 'disabled') this.disabled = true;
  }

  getAttribute(name: string): string | null {
    return this.attributes.has(name) ? this.attributes.get(name)! : null;
  }

  removeAttribute(name: string): void {
    this.attributes.delete(name);
    if (name === 'disabled') this.disabled = false;
  }

  hasAttribute(name: string): boolean {
    return this.attributes.has(name);
  }

  addEventListener(event: string, listener: (e: any) => void): void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, []);
    }
    this.listeners.get(event)!.push(listener);
  }

  removeEventListener(event: string, listener: (e: any) => void): void {
    if (!this.listeners.has(event)) return;
    const arr = this.listeners.get(event)!.filter((l) => l !== listener);
    this.listeners.set(event, arr);
  }

  dispatchEvent(event: any): boolean {
    const ev =
      typeof event === 'string'
        ? { type: event, target: this, currentTarget: this, stopPropagation: () => {} }
        : event;
    if (!ev.target) ev.target = this;
    if (!ev.currentTarget) ev.currentTarget = this;
    if (!ev.stopPropagation) ev.stopPropagation = () => {};
    const handlers = this.listeners.get(ev.type) || [];
    for (const h of handlers) {
      h(ev);
    }
    return true;
  }

  click(): void {
    this.dispatchEvent('click');
  }

  _matchesSingle(sel: string): boolean {
    if (sel === '*') return true;
    if (sel.startsWith('.')) {
      const classes = sel.slice(1).split('.');
      return classes.every((c) => this.classList.contains(c));
    }
    if (sel.startsWith('#')) return this.id === sel.slice(1);
    if (sel.startsWith('[') && sel.endsWith(']')) {
      const inside = sel.slice(1, -1);
      if (inside.includes('=')) {
        const [k, v] = inside.split('=').map((s) => s.trim().replace(/^['"]|['"]$/g, ''));
        return this.getAttribute(k) === v;
      }
      return this.hasAttribute(inside);
    }
    return this.tagName.toLowerCase() === sel.toLowerCase();
  }

  querySelectorAll(selector: string): MockElement[] {
    const parts = selector.trim().split(/\s+/);
    let current: MockElement[] = [];
    if (this._matchesSingle(parts[0])) {
      current.push(this);
    }
    this._collectDescendantsMatching(this, parts[0], current);

    for (let i = 1; i < parts.length; i++) {
      const next: MockElement[] = [];
      for (const el of current) {
        this._collectDescendantsMatching(el, parts[i], next);
      }
      current = next;
    }
    return current;
  }

  querySelector(selector: string): MockElement | null {
    const all = this.querySelectorAll(selector);
    return all.length > 0 ? all[0] : null;
  }

  private _collectDescendantsMatching(root: MockElement, sel: string, result: MockElement[]): void {
    for (const child of root.children) {
      if (child._matchesSingle(sel) && !result.includes(child)) {
        result.push(child);
      }
      this._collectDescendantsMatching(child, sel, result);
    }
  }
}

class MockDocument {
  public body: MockElement = new MockElement('BODY');

  createElement(tagName: string): MockElement {
    return new MockElement(tagName);
  }

  getElementById(id: string): MockElement | null {
    return this.body.querySelector(`#${id}`);
  }
}

// Install mock DOM on globalThis before component imports
const mockDoc = new MockDocument();
(globalThis as any).document = mockDoc;
(globalThis as any).HTMLElement = MockElement;
(globalThis as any).HTMLButtonElement = MockElement;

// ---------------------------------------------------------------------------
// Imports of components to test
// ---------------------------------------------------------------------------
import { PulseRing } from '../src/components/PulseRing.js';
import { Controls } from '../src/components/Controls.js';
import { ExpandedCard } from '../src/components/ExpandedCard.js';
import { DockPill, type DockPosition } from '../src/components/DockPill.js';
import type { CaffeineStatusResponse, CaffeineState } from '../src/types/state.js';

// ---------------------------------------------------------------------------
// Test Suites
// ---------------------------------------------------------------------------

describe('PulseRing', () => {
  it('creates DOM element with .pulse-ring and default idle state', () => {
    const ring = new PulseRing();
    const el = ring.render();
    expect(el).toBeDefined();
    expect(el.classList.contains('pulse-ring')).toBe(true);
    expect(el.classList.contains('state-idle')).toBe(true);
    expect(el.getAttribute('data-state')).toBe('idle');
  });

  it('initializes with specific state', () => {
    const ring = new PulseRing('active');
    expect(ring.element.classList.contains('state-active')).toBe(true);
    expect(ring.element.classList.contains('state-idle')).toBe(false);
  });

  it('transitions state classes across active, cooldown, override, and idle', () => {
    const ring = new PulseRing('idle');
    const states: CaffeineState[] = ['active', 'cooldown', 'override', 'idle', 'approval'];

    for (const st of states) {
      ring.update(st);
      expect(ring.element.classList.contains(`state-${st}`)).toBe(true);
      expect(ring.element.getAttribute('data-state')).toBe(st);

      // Verify other state classes are removed
      for (const other of states) {
        if (other !== st) {
          expect(ring.element.classList.contains(`state-${other}`)).toBe(false);
        }
      }
    }
  });

  it('updates container classes when container is provided or parent exists', () => {
    const container = new MockElement('div') as unknown as HTMLElement;
    const ring = new PulseRing('idle');
    ring.update('active', container);
    expect(container.classList.contains('state-active')).toBe(true);

    ring.update('cooldown', container);
    expect(container.classList.contains('state-cooldown')).toBe(true);
    expect(container.classList.contains('state-active')).toBe(false);
  });
});

describe('Controls', () => {
  it('renders all four buttons with correct text and glassmorphic styling', () => {
    const controls = new Controls();
    const el = controls.render();

    expect(el.classList.contains('card-actions')).toBe(true);

    const forceBtn = el.querySelector<HTMLButtonElement>('.btn-force-awake');
    expect(forceBtn).toBeDefined();
    expect(forceBtn?.textContent).toBe('Force Awake');
    expect(forceBtn?.style.color).toBe('var(--accent-blue, #2979ff)');

    const dismissBtn = el.querySelector('.btn-dismiss');
    expect(dismissBtn).toBeDefined();
    expect(dismissBtn?.textContent).toBe('Dismiss to Idle');

    const sleepBtn = el.querySelector('.btn-sleep');
    expect(sleepBtn).toBeDefined();
    expect(sleepBtn?.textContent).toBe('Sleep Now');

    const cancelBtn = el.querySelector('.btn-cancel');
    expect(cancelBtn).toBeDefined();
    expect(cancelBtn?.textContent).toBe('Cancel');
  });

  it('triggers callbacks when buttons are clicked', () => {
    const onForceAwake = vi.fn();
    const onCancel = vi.fn();
    const onDismiss = vi.fn();
    const onSleep = vi.fn();

    const controls = new Controls({
      onForceAwake,
      onCancel,
      onDismiss,
      onSleep,
    });

    controls.forceAwakeBtn.click();
    expect(onForceAwake).toHaveBeenCalledTimes(1);

    controls.dismissBtn.click();
    expect(onDismiss).toHaveBeenCalledTimes(1);

    controls.sleepBtn.click();
    expect(onSleep).toHaveBeenCalledTimes(1);

    controls.cancelBtn.click();
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('guarded failure: handles clicks safely when callbacks are omitted', () => {
    const controls = new Controls({});
    expect(() => {
      controls.forceAwakeBtn.click();
      controls.dismissBtn.click();
      controls.sleepBtn.click();
      controls.cancelBtn.click();
    }).not.toThrow();
  });

  it('updates state dynamically and adjusts button attributes', () => {
    const controls = new Controls();
    controls.updateState('override');
    expect(controls.element.getAttribute('data-state')).toBe('override');
    expect(controls.forceAwakeBtn.classList.contains('active')).toBe(true);

    controls.updateState('idle');
    expect(controls.element.getAttribute('data-state')).toBe('idle');
    expect(controls.forceAwakeBtn.classList.contains('active')).toBe(false);
    expect(controls.dismissBtn.getAttribute('disabled')).toBe('true');
  });
});

describe('ExpandedCard', () => {
  const mockStatus: CaffeineStatusResponse = {
    state: 'active',
    active_inhibitor: 'org.freedesktop.login1',
    lock_held: true,
    cooldown_remaining_seconds: 0,
    override_remaining_seconds: 0,
    active_leases: [
      {
        session_id: 'session-xyz-123',
        model: 'gemini-3.8-flash',
        started_at: '2026-11-06T14:00:00Z',
        state: 'RUNNING',
        description: 'Auto-lifecycle test execution',
      },
    ],
    hardware: {
      battery_percent: 85,
      is_charging: true,
      cpu_temp_c: 47.5,
      lid_closed: false,
    },
    circuit_breakers: {
      battery_tripped: false,
      thermal_tripped: false,
      lid_tripped: false,
      ceiling_tripped: false,
    },
  };

  it('renders active leases with model/session_id badge and details', () => {
    const card = new ExpandedCard();
    card.render(mockStatus);

    const leaseBadge = card.element.querySelector('.lease-badge');
    expect(leaseBadge).toBeDefined();
    expect(leaseBadge?.textContent).toBe('gemini-3.8-flash');

    const leaseStatus = card.element.querySelector('.lease-status');
    expect(leaseStatus?.textContent).toContain('RUNNING');
    expect(leaseStatus?.textContent).toContain('Auto-lifecycle test execution');
  });

  it('renders safe fallback when active leases is empty', () => {
    const card = new ExpandedCard();
    card.render({ ...mockStatus, active_leases: [] });

    const empty = card.element.querySelector('.empty-leases');
    expect(empty).toBeDefined();
    expect(empty?.textContent).toContain('No active leases');
  });

  it('renders inhibitor backend and sleep lock status (LOCKED / UNLOCKED)', () => {
    const card = new ExpandedCard();
    card.render(mockStatus);

    const inhibitorVal = card.element.querySelector('.inhibitor-value');
    expect(inhibitorVal?.textContent).toBe('org.freedesktop.login1');

    const lockVal = card.element.querySelector('.lock-held-value');
    expect(lockVal?.textContent).toBe('LOCKED');

    // Test unlocked
    card.update({ ...mockStatus, lock_held: false });
    const lockValAfter = card.element.querySelector('.lock-held-value');
    expect(lockValAfter?.textContent).toBe('UNLOCKED');
  });

  it('renders hardware metrics when present', () => {
    const card = new ExpandedCard();
    card.render(mockStatus);

    const batVal = card.element.querySelector('.hardware-battery');
    expect(batVal?.textContent).toBe('85% (charging)');

    const tempVal = card.element.querySelector('.hardware-temp');
    expect(tempVal?.textContent).toBe('47.5°C');

    const lidVal = card.element.querySelector('.hardware-lid');
    expect(lidVal?.textContent).toBe('Open');
  });

  it('safely handles missing hardware metrics (guarded fallback)', () => {
    const card = new ExpandedCard();
    card.render({ ...mockStatus, hardware: undefined });

    const hwSection = card.element.querySelector('.hardware-section');
    expect(hwSection).toBeNull();
  });

  it('displays warning badges when circuit breakers are tripped', () => {
    const card = new ExpandedCard();
    card.render({
      ...mockStatus,
      circuit_breakers: {
        battery_tripped: true,
        thermal_tripped: true,
        lid_tripped: true,
        ceiling_tripped: true,
      },
    });

    expect(card.element.querySelector('.breaker-battery')).toBeDefined();
    expect(card.element.querySelector('.breaker-thermal')).toBeDefined();
    expect(card.element.querySelector('.breaker-lid')).toBeDefined();
    expect(card.element.querySelector('.breaker-ceiling')).toBeDefined();
  });

  it('does not display circuit breaker section when none are tripped', () => {
    const card = new ExpandedCard();
    card.render(mockStatus);

    const cbSection = card.element.querySelector('.circuit-breakers-section');
    expect(cbSection).toBeNull();
  });

  it('displays cooldown buffer and progress bar during cooldown state', () => {
    const card = new ExpandedCard();
    card.render({
      ...mockStatus,
      state: 'cooldown',
      cooldown_remaining_seconds: 30,
    });

    const cdCountdown = card.element.querySelector('.cooldown-countdown');
    expect(cdCountdown).toBeDefined();
    expect(cdCountdown?.textContent).toBe('30s');

    const barFill = card.element.querySelector<HTMLElement>('.cooldown-bar-fill');
    expect(barFill).toBeDefined();
    expect(barFill?.style.width).toBeDefined();
  });

  it('toggles open/close state and invokes onToggle callback', () => {
    const onToggle = vi.fn();
    const card = new ExpandedCard({ onToggle });

    expect(card.isOpen).toBe(false);
    expect(card.element.style.display).toBe('none');

    // Toggle open
    const openState = card.toggle();
    expect(openState).toBe(true);
    expect(card.isOpen).toBe(true);
    expect(card.element.classList.contains('visible')).toBe(true);
    expect(card.element.style.display).toBe('flex');
    expect(onToggle).toHaveBeenCalledWith(true);

    // Toggle closed
    const closedState = card.toggle();
    expect(closedState).toBe(false);
    expect(card.isOpen).toBe(false);
    expect(card.element.classList.contains('visible')).toBe(false);
    expect(card.element.style.display).toBe('none');
    expect(onToggle).toHaveBeenCalledWith(false);

    // Force open
    card.toggle(true);
    expect(card.isOpen).toBe(true);
    expect(card.element.style.display).toBe('flex');

    // Force closed
    card.toggle(false);
    expect(card.isOpen).toBe(false);
    expect(card.element.style.display).toBe('none');
  });

  it('embeds controls and forwards action callbacks', () => {
    const onForceAwake = vi.fn();
    const card = new ExpandedCard({ onForceAwake });
    card.render(mockStatus);

    const forceBtn = card.element.querySelector<HTMLButtonElement>('.btn-force-awake');
    expect(forceBtn).toBeDefined();
    forceBtn?.click();
    expect(onForceAwake).toHaveBeenCalledTimes(1);
  });
});

describe('DockPill', () => {
  const activeStatus: CaffeineStatusResponse = {
    state: 'active',
    active_inhibitor: 'org.freedesktop.login1',
    lock_held: true,
    cooldown_remaining_seconds: 0,
    override_remaining_seconds: 0,
    active_leases: [
      {
        session_id: 'subagent-a',
        model: 'AGY',
        state: 'RUNNING',
      },
    ],
  };

  it('renders dock pill with default position and elements', () => {
    const pill = new DockPill();
    pill.render(activeStatus);

    expect(pill.element.classList.contains('hud-pill-container')).toBe(true);
    expect(pill.element.classList.contains('pos-top-center')).toBe(true);
    expect(pill.element.classList.contains('state-active')).toBe(true);

    expect(pill.pillElement.classList.contains('hud-pill')).toBe(true);
    expect(pill.pulseRing).toBeDefined();
    expect(pill.agentTagElement.textContent).toBe('AGY');
    expect(pill.stateLabelElement.textContent).toBe('ACTIVE');
    expect(pill.statusBadgeElement.textContent).toBe('SLEEP LOCKED');
  });

  it('switches screen docking positions cleanly', () => {
    const pill = new DockPill({ position: 'pos-top-center' });

    pill.setPosition('pos-top-right');
    expect(pill.position).toBe('pos-top-right');
    expect(pill.element.classList.contains('pos-top-right')).toBe(true);
    expect(pill.element.classList.contains('pos-top-center')).toBe(false);

    pill.setPosition('pos-bottom-right');
    expect(pill.position).toBe('pos-bottom-right');
    expect(pill.element.classList.contains('pos-bottom-right')).toBe(true);
    expect(pill.element.classList.contains('pos-top-right')).toBe(false);
  });

  it('clicking pillElement toggles expanded card', () => {
    const pill = new DockPill();
    pill.render(activeStatus);

    expect(pill.expandedCard.isOpen).toBe(false);

    pill.pillElement.click();
    expect(pill.expandedCard.isOpen).toBe(true);

    pill.pillElement.click();
    expect(pill.expandedCard.isOpen).toBe(false);
  });

  it('updates lifecycle: updates badges and labels across states', () => {
    const pill = new DockPill();

    // 1. Active state
    pill.update(activeStatus);
    expect(pill.element.classList.contains('state-active')).toBe(true);
    expect(pill.statusBadgeElement.textContent).toBe('SLEEP LOCKED');

    // 2. Cooldown state
    pill.update({
      ...activeStatus,
      state: 'cooldown',
      cooldown_remaining_seconds: 45,
    });
    expect(pill.element.classList.contains('state-cooldown')).toBe(true);
    expect(pill.statusBadgeElement.textContent).toBe('COOLDOWN 45s');
    expect(pill.cooldownIndicatorElement.textContent).toBe('45s');
    expect(pill.cooldownIndicatorElement.style.display).toBe('inline');

    // 3. Override state
    pill.update({
      ...activeStatus,
      state: 'override',
      cooldown_remaining_seconds: 0,
    });
    expect(pill.element.classList.contains('state-override')).toBe(true);
    expect(pill.statusBadgeElement.textContent).toBe('MANUAL LOCK');

    // 4. Idle state
    pill.update({
      ...activeStatus,
      state: 'idle',
      lock_held: false,
      active_leases: [],
    });
    expect(pill.element.classList.contains('state-idle')).toBe(true);
    expect(pill.statusBadgeElement.textContent).toBe('IDLE');
    expect(pill.agentTagElement.textContent).toBe('AGENTKIT'); // fallback
  });

  it('forwards action callbacks to embedded controls', () => {
    const onForceAwake = vi.fn();
    const onSleep = vi.fn();
    const pill = new DockPill({
      onAction: { onForceAwake, onSleep },
    });
    pill.render(activeStatus);

    const forceBtn = pill.element.querySelector<HTMLButtonElement>('.btn-force-awake');
    forceBtn?.click();
    expect(onForceAwake).toHaveBeenCalledTimes(1);

    const sleepBtn = pill.element.querySelector<HTMLButtonElement>('.btn-sleep');
    sleepBtn?.click();
    expect(onSleep).toHaveBeenCalledTimes(1);
  });
});
