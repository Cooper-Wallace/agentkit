import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

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

  remove(): void {
    if (this.parentElement) {
      this.parentElement.removeChild(this);
    }
  }

  contains(child: MockElement | null): boolean {
    let curr = child;
    while (curr) {
      if (curr === this) return true;
      curr = curr.parentElement;
    }
    return false;
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

  getAttributeNames(): string[] {
    return Array.from(this.attributes.keys());
  }

  hasChildNodes(): boolean {
    return this.children.length > 0;
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
  public listeners: Map<string, Array<(e: any) => void>> = new Map();

  createElement(tagName: string): MockElement {
    return new MockElement(tagName);
  }

  getElementById(id: string): MockElement | null {
    if (this.body.id === id) return this.body;
    return this.body.querySelector(`#${id}`);
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
        ? { type: event, target: this.body, stopPropagation: () => {} }
        : event;
    const handlers = this.listeners.get(ev.type) || [];
    for (const h of handlers) {
      h(ev);
    }
    return true;
  }
}

class MockEventSource {
  public static instances: MockEventSource[] = [];
  public static autoOpen: boolean = true;
  public url: string;
  public readyState: number = 0; // 0 = CONNECTING, 1 = OPEN, 2 = CLOSED
  public onopen: (() => void) | null = null;
  public onmessage: ((ev: MessageEvent) => void) | null = null;
  public onerror: ((ev: any) => void) | null = null;
  private listeners: Map<string, ((ev: any) => void)[]> = new Map();

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
    if (MockEventSource.autoOpen) {
      this.readyState = 1;
      setTimeout(() => {
        if (this.onopen) this.onopen();
      }, 0);
    }
  }

  addEventListener(type: string, listener: (ev: any) => void) {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, []);
    }
    this.listeners.get(type)!.push(listener);
  }

  removeEventListener(type: string, listener: (ev: any) => void) {
    const list = this.listeners.get(type);
    if (list) {
      this.listeners.set(
        type,
        list.filter((l) => l !== listener)
      );
    }
  }

  dispatchEvent(type: string, data: any) {
    const event = { type, data: typeof data === 'string' ? data : JSON.stringify(data) };
    if (type === 'message' && this.onmessage) {
      this.onmessage(event as MessageEvent);
    }
    const handlers = this.listeners.get(type) || [];
    for (const h of handlers) {
      h(event);
    }
  }

  close() {
    this.readyState = 2;
  }
}

// Install mock DOM and globals before importing application module
const mockDoc = new MockDocument();
(globalThis as any).document = mockDoc;
(globalThis as any).HTMLElement = MockElement;
(globalThis as any).HTMLButtonElement = MockElement;
(globalThis as any).EventSource = MockEventSource as any;

// Global fetch mock
const fetchMock = vi.fn();
(globalThis as any).fetch = fetchMock;

// ---------------------------------------------------------------------------
// Imports
// ---------------------------------------------------------------------------
import { mountCaffeineDock } from '../src/main.js';
import type { CaffeineStatusResponse } from '../src/types/state.js';

// ---------------------------------------------------------------------------
// Test Suites
// ---------------------------------------------------------------------------

describe('mountCaffeineDock Application Integration', () => {
  const activeStatusPayload: CaffeineStatusResponse = {
    state: 'active',
    active_inhibitor: 'org.freedesktop.login1',
    lock_held: true,
    cooldown_remaining_seconds: 0,
    override_remaining_seconds: 0,
    active_leases: [
      {
        session_id: 'subagent-42',
        model: 'GEMINI-3.8-FLASH',
        state: 'RUNNING',
        started_at: '2026-11-06T22:00:00Z',
        description: 'Auto-lifecycle test execution',
      },
    ],
    hardware: {
      battery_percent: 92,
      is_charging: true,
      cpu_temp_c: 48,
      lid_closed: false,
    },
  };

  beforeEach(() => {
    vi.clearAllMocks();
    MockEventSource.instances = [];
    MockEventSource.autoOpen = true;
    mockDoc.body = new MockElement('BODY');
    mockDoc.listeners.clear();

    // Default healthy fetch
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        state: 'idle',
        active_inhibitor: 'none',
        lock_held: false,
        cooldown_remaining_seconds: 0,
        override_remaining_seconds: 0,
        active_leases: [],
      }),
      text: async () => '',
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('mounts pill into container and calls initial getStatus', async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => activeStatusPayload,
      text: async () => '',
    });

    const container = mockDoc.createElement('div');
    const dock = mountCaffeineDock(container as unknown as HTMLElement);

    expect(container.children.includes(dock.pill.element as any)).toBe(true);
    expect(fetchMock).toHaveBeenCalledWith('http://127.0.0.1:18820/status', expect.anything());

    await vi.waitFor(() => {
      expect(dock.pill.element.classList.contains('state-active')).toBe(true);
      expect(dock.pill.stateLabelElement.textContent).toBe('ACTIVE');
    });

    expect(dock.pill.statusBadgeElement.textContent).toBe('SLEEP LOCKED');
    expect(dock.pill.agentTagElement.textContent).toBe('GEMINI-3.8-FLASH');

    dock.destroy();
  });

  it('receives stream updates and reactively updates pill', async () => {
    const container = mockDoc.createElement('div');
    const dock = mountCaffeineDock(container as unknown as HTMLElement);

    expect(MockEventSource.instances).toHaveLength(1);
    const mockES = MockEventSource.instances[0];

    // 1. Dispatch cooldown state change event from daemon
    mockES.dispatchEvent('state_change', {
      state: 'COOLDOWN',
      cooldown_remaining: 38,
      lock_held: true,
    });

    expect(dock.pill.element.classList.contains('state-cooldown')).toBe(true);
    expect(dock.pill.stateLabelElement.textContent).toBe('COOLDOWN');
    expect(dock.pill.cooldownIndicatorElement.textContent).toBe('38s');
    expect(dock.pill.cooldownIndicatorElement.style.display).toBe('inline');
    expect(dock.pill.statusBadgeElement.textContent).toBe('COOLDOWN 38s');

    // 2. Dispatch override event
    mockES.dispatchEvent('state_change', {
      state: 'OVERRIDE',
      override_remaining: 1800,
      lock_held: true,
    });

    expect(dock.pill.element.classList.contains('state-override')).toBe(true);
    expect(dock.pill.stateLabelElement.textContent).toBe('OVERRIDE');
    expect(dock.pill.statusBadgeElement.textContent).toBe('MANUAL LOCK');

    // 3. Dispatch idle heartbeat
    mockES.dispatchEvent('heartbeat', {
      state: 'IDLE',
      lock_held: false,
      leases: [],
    });

    expect(dock.pill.element.classList.contains('state-idle')).toBe(true);
    expect(dock.pill.stateLabelElement.textContent).toBe('IDLE');
    expect(dock.pill.statusBadgeElement.textContent).toBe('IDLE');

    dock.destroy();
  });

  it('wires action buttons (Force Awake, Dismiss, Sleep, Cancel) to call client methods', async () => {
    const container = mockDoc.createElement('div');
    const dock = mountCaffeineDock(container as unknown as HTMLElement);

    const overrideSpy = vi.spyOn(dock.client, 'override').mockResolvedValue(true);
    const cancelSpy = vi.spyOn(dock.client, 'cancel').mockResolvedValue(true);
    const notifySpy = vi.spyOn(dock.client, 'notify').mockResolvedValue(true);
    const getStatusSpy = vi.spyOn(dock.client, 'getStatus').mockResolvedValue(activeStatusPayload);

    // 1. Force Awake
    dock.pill.expandedCard.controls.forceAwakeBtn.click();
    expect(overrideSpy).toHaveBeenCalledTimes(1);
    expect(overrideSpy).toHaveBeenCalledWith();

    // 2. Dismiss to Idle
    dock.pill.expandedCard.controls.dismissBtn.click();
    expect(notifySpy).toHaveBeenCalledTimes(1);
    expect(notifySpy).toHaveBeenCalledWith('user-dismiss', 'idle');

    // 3. Sleep Now (calls override('0s'))
    dock.pill.expandedCard.controls.sleepBtn.click();
    expect(overrideSpy).toHaveBeenCalledWith('0s');

    // 4. Cancel
    dock.pill.expandedCard.controls.cancelBtn.click();
    expect(cancelSpy).toHaveBeenCalledTimes(1);

    await vi.waitFor(() => {
      expect(getStatusSpy).toHaveBeenCalled();
    });

    dock.destroy();
  });

  it('closes expanded card on Escape key and click-outside', () => {
    const container = mockDoc.createElement('div');
    mockDoc.body.appendChild(container);
    const dock = mountCaffeineDock(container as unknown as HTMLElement);

    // Open card
    dock.pill.expandedCard.toggle(true);
    expect(dock.pill.expandedCard.isOpen).toBe(true);

    // 1. Escape key closes card
    mockDoc.dispatchEvent({ type: 'keydown', key: 'Escape' });
    expect(dock.pill.expandedCard.isOpen).toBe(false);

    // Re-open card
    dock.pill.expandedCard.toggle(true);
    expect(dock.pill.expandedCard.isOpen).toBe(true);

    // 2. Click outside closes card
    const outsideEl = mockDoc.createElement('div');
    mockDoc.body.appendChild(outsideEl);
    mockDoc.dispatchEvent({ type: 'click', target: outsideEl });
    expect(dock.pill.expandedCard.isOpen).toBe(false);

    // 3. Click inside pill element does not close card
    dock.pill.expandedCard.toggle(true);
    expect(dock.pill.expandedCard.isOpen).toBe(true);
    mockDoc.dispatchEvent({ type: 'click', target: dock.pill.pillElement });
    expect(dock.pill.expandedCard.isOpen).toBe(true);

    dock.destroy();
  });

  it('destroys cleanly by disconnecting stream, removing listeners, and removing DOM element', () => {
    const container = mockDoc.createElement('div');
    const dock = mountCaffeineDock(container as unknown as HTMLElement);

    expect(container.children.includes(dock.pill.element as any)).toBe(true);

    const streamDisconnectSpy = vi.spyOn(dock.stream, 'disconnect');

    dock.destroy();

    expect(streamDisconnectSpy).toHaveBeenCalledTimes(1);
    expect(container.children.includes(dock.pill.element as any)).toBe(false);

    // Event listeners must be removed from document
    dock.pill.expandedCard.toggle(true);
    mockDoc.dispatchEvent({ type: 'keydown', key: 'Escape' });
    expect(dock.pill.expandedCard.isOpen).toBe(true); // unaffected

    const outsideEl = mockDoc.createElement('div');
    mockDoc.dispatchEvent({ type: 'click', target: outsideEl });
    expect(dock.pill.expandedCard.isOpen).toBe(true); // unaffected
  });

  it('gracefully degrades to idle when daemon is offline without throwing', async () => {
    fetchMock.mockRejectedValueOnce(new Error('Failed to fetch from 127.0.0.1:18820: connection refused'));

    const container = mockDoc.createElement('div');
    let dock: any;
    expect(() => {
      dock = mountCaffeineDock(container as unknown as HTMLElement);
    }).not.toThrow();

    await vi.waitFor(() => {
      expect(dock.pill.element.classList.contains('state-idle')).toBe(true);
      expect(dock.pill.stateLabelElement.textContent).toBe('IDLE');
      expect(dock.pill.statusBadgeElement.textContent).toBe('IDLE');
    });

    dock.destroy();
  });

  it('configures custom position and autoConnect options', () => {
    const container = mockDoc.createElement('div');
    const dock = mountCaffeineDock(container as unknown as HTMLElement, {
      position: 'pos-top-right',
      autoConnect: false,
    });

    expect(dock.pill.position).toBe('pos-top-right');
    expect(dock.pill.element.classList.contains('pos-top-right')).toBe(true);
    expect(dock.stream.isConnected).toBe(false);
    expect(MockEventSource.instances).toHaveLength(0); // autoConnect false did not connect stream

    dock.destroy();
  });

  it('configures custom baseUrl for daemon client and stream', () => {
    const container = mockDoc.createElement('div');
    const dock = mountCaffeineDock(container as unknown as HTMLElement, {
      baseUrl: 'http://192.168.1.50:9000',
    });

    expect(fetchMock).toHaveBeenCalledWith('http://192.168.1.50:9000/status', expect.anything());
    expect(MockEventSource.instances[0].url).toBe('http://192.168.1.50:9000/events');

    dock.destroy();
  });

  it('guarded failure: handles action failure and refresh error gracefully', async () => {
    const container = mockDoc.createElement('div');
    const dock = mountCaffeineDock(container as unknown as HTMLElement);

    // Make override reject
    vi.spyOn(dock.client, 'override').mockRejectedValue(new Error('Daemon write failure'));

    expect(() => {
      dock.pill.expandedCard.controls.forceAwakeBtn.click();
    }).not.toThrow();

    dock.destroy();
  });

  it('handles stream reconnection errors gracefully', () => {
    const container = mockDoc.createElement('div');
    const dock = mountCaffeineDock(container as unknown as HTMLElement);

    const mockES = MockEventSource.instances[0];
    expect(() => {
      mockES.onerror?.(new Error('Network offline'));
    }).not.toThrow();

    dock.destroy();
  });

  it('verifies click on document does nothing when expandedCard is closed', () => {
    const container = mockDoc.createElement('div');
    mockDoc.body.appendChild(container);
    const dock = mountCaffeineDock(container as unknown as HTMLElement);

    expect(dock.pill.expandedCard.isOpen).toBe(false);

    const outsideEl = mockDoc.createElement('div');
    mockDoc.body.appendChild(outsideEl);
    expect(() => {
      mockDoc.dispatchEvent({ type: 'click', target: outsideEl });
    }).not.toThrow();

    expect(dock.pill.expandedCard.isOpen).toBe(false);

    dock.destroy();
  });
});
