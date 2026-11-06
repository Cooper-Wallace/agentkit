import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { CaffeineEventStream } from '../src/api/stream.js';
import type { CaffeineStatusResponse } from '../src/types/state.js';

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

  triggerOpen() {
    this.readyState = 1;
    if (this.onopen) this.onopen();
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

describe('CaffeineEventStream', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    MockEventSource.instances = [];
    MockEventSource.autoOpen = true;
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('connects to configured stream endpoint and receives status events', () => {
    const stream = new CaffeineEventStream({
      baseUrl: 'http://127.0.0.1:18820',
      EventSourceClass: MockEventSource as unknown as typeof EventSource,
    });

    const received: CaffeineStatusResponse[] = [];
    stream.connect((status) => {
      received.push(status);
    });

    expect(MockEventSource.instances).toHaveLength(1);
    const mockES = MockEventSource.instances[0];
    expect(mockES.url).toBe('http://127.0.0.1:18820/events');

    // Simulate state_change event pushed by Go daemon
    const testPayload = {
      state: 'ACTIVE',
      inhibited: true,
      leases: [
        {
          session_id: 'subagent-42',
          agent_name: 'test-agent',
          task: 'verifying stream',
          state: 'ACTIVE',
        },
      ],
      cooldown_remaining: 0,
      override_remaining: 0,
    };

    mockES.dispatchEvent('state_change', testPayload);

    expect(received).toHaveLength(1);
    expect(received[0].state).toBe('active');
    expect(received[0].lock_held).toBe(true);
    expect(received[0].active_leases).toHaveLength(1);
    expect(received[0].active_leases[0].session_id).toBe('subagent-42');

    stream.disconnect();
    expect(mockES.readyState).toBe(2);
  });

  it('handles standard message event and heartbeat event', () => {
    const stream = new CaffeineEventStream({
      baseUrl: 'http://127.0.0.1:18820',
      EventSourceClass: MockEventSource as unknown as typeof EventSource,
    });

    const received: CaffeineStatusResponse[] = [];
    stream.connect((status) => {
      received.push(status);
    });

    const mockES = MockEventSource.instances[0];

    // Standard message
    mockES.dispatchEvent('message', { state: 'COOLDOWN', cooldown_remaining: 10 });
    expect(received).toHaveLength(1);
    expect(received[0].state).toBe('cooldown');
    expect(received[0].cooldown_remaining_seconds).toBe(10);

    // Heartbeat
    mockES.dispatchEvent('heartbeat', { state: 'IDLE' });
    expect(received).toHaveLength(2);
    expect(received[1].state).toBe('idle');

    stream.disconnect();
  });

  it('triggers error callback and reconnects with exponential backoff on connection error', () => {
    MockEventSource.autoOpen = false; // Disable auto open to simulate broken connection
    const stream = new CaffeineEventStream({
      baseUrl: 'http://127.0.0.1:18820',
      initialBackoffMs: 500,
      backoffMultiplier: 2.0,
      maxBackoffMs: 2000,
      EventSourceClass: MockEventSource as unknown as typeof EventSource,
    });

    const errors: any[] = [];
    stream.connect(
      () => {},
      (err) => errors.push(err)
    );

    expect(MockEventSource.instances).toHaveLength(1);
    const mockES1 = MockEventSource.instances[0];

    // Simulate error on first instance
    mockES1.onerror?.(new Error('Connection lost'));
    expect(errors).toHaveLength(1);
    expect(mockES1.readyState).toBe(2); // closed

    // Before timer advances, no new instance
    expect(MockEventSource.instances).toHaveLength(1);

    // Advance by backoff delay (500ms)
    vi.advanceTimersByTime(500);
    expect(MockEventSource.instances).toHaveLength(2);

    // Simulate second error (backoff should be 1000ms)
    const mockES2 = MockEventSource.instances[1];
    mockES2.onerror?.(new Error('Connection failed again'));
    expect(errors).toHaveLength(2);

    vi.advanceTimersByTime(500);
    expect(MockEventSource.instances).toHaveLength(2); // not yet reconnected

    vi.advanceTimersByTime(500); // 1000ms total
    expect(MockEventSource.instances).toHaveLength(3);

    // Now simulate successful open on third attempt: backoff resets to initial
    const mockES3 = MockEventSource.instances[2];
    mockES3.triggerOpen();
    expect(stream.reconnectDelay).toBe(500);

    stream.disconnect();
  });

  it('stops reconnecting when explicitly disconnected', () => {
    const stream = new CaffeineEventStream({
      baseUrl: 'http://127.0.0.1:18820',
      initialBackoffMs: 500,
      EventSourceClass: MockEventSource as unknown as typeof EventSource,
    });

    stream.connect(() => {});
    expect(MockEventSource.instances).toHaveLength(1);

    stream.disconnect();
    vi.advanceTimersByTime(2000);
    expect(MockEventSource.instances).toHaveLength(1);
  });

  describe('Guarded Failure Handling', () => {
    it('gracefully handles malformed JSON without crashing the stream', () => {
      const stream = new CaffeineEventStream({
        baseUrl: 'http://127.0.0.1:18820',
        EventSourceClass: MockEventSource as unknown as typeof EventSource,
      });

      const errors: any[] = [];
      const received: any[] = [];
      stream.connect(
        (st) => received.push(st),
        (err) => errors.push(err)
      );

      const mockES = MockEventSource.instances[0];
      mockES.dispatchEvent('message', 'INVALID_JSON_BODY');

      expect(errors).toHaveLength(1);
      expect(received).toHaveLength(0);

      stream.disconnect();
    });

    it('safely handles missing EventSource in unsupported environment', () => {
      const stream = new CaffeineEventStream({
        EventSourceClass: undefined,
      });

      const errors: any[] = [];
      stream.connect(
        () => {},
        (err) => errors.push(err)
      );

      expect(errors).toHaveLength(1);
      expect(errors[0].message).toContain('EventSource is not supported');
    });
  });
});
