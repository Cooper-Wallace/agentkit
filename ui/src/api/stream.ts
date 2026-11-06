import type { CaffeineStatusResponse } from '../types/state.js';
import { normalizeCaffeineStatus } from './client.js';

export interface CaffeineStreamConfig {
  baseUrl?: string;
  streamPath?: string;
  initialBackoffMs?: number;
  maxBackoffMs?: number;
  backoffMultiplier?: number;
  EventSourceClass?: typeof EventSource;
}

export type StatusCallback = (status: CaffeineStatusResponse) => void;
export type ErrorCallback = (err: any) => void;

export class CaffeineEventStream {
  private readonly streamUrl: string;
  private readonly initialBackoffMs: number;
  private readonly maxBackoffMs: number;
  private readonly backoffMultiplier: number;
  private readonly EventSourceClass?: typeof EventSource;

  private eventSource: EventSource | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private currentBackoffMs: number;
  private isManuallyClosed = false;

  private onStatusCallback: StatusCallback | null = null;
  private onErrorCallback: ErrorCallback | null = null;

  constructor(config: CaffeineStreamConfig = {}) {
    const base = (config.baseUrl ?? 'http://127.0.0.1:18820').replace(/\/+$/, '');
    const path = config.streamPath ?? '/events';
    this.streamUrl = `${base}${path.startsWith('/') ? path : `/${path}`}`;
    this.initialBackoffMs = config.initialBackoffMs ?? 1000;
    this.maxBackoffMs = config.maxBackoffMs ?? 15000;
    this.backoffMultiplier = config.backoffMultiplier ?? 1.5;
    this.currentBackoffMs = this.initialBackoffMs;
    this.EventSourceClass = config.EventSourceClass ?? globalThis.EventSource;
  }

  public get isConnected(): boolean {
    return this.eventSource !== null && this.eventSource.readyState === 1; // 1 = OPEN
  }

  public get reconnectDelay(): number {
    return this.currentBackoffMs;
  }

  public connect(onStatus: StatusCallback, onError?: ErrorCallback): void {
    this.onStatusCallback = onStatus;
    this.onErrorCallback = onError ?? null;
    this.isManuallyClosed = false;
    this.currentBackoffMs = this.initialBackoffMs;

    this.initiateConnection();
  }

  private initiateConnection(): void {
    this.cleanupEventSource();

    if (this.isManuallyClosed) {
      return;
    }

    const ES = this.EventSourceClass ?? globalThis.EventSource;
    if (!ES) {
      const err = new Error('EventSource is not supported in this environment');
      this.onErrorCallback?.(err);
      return;
    }

    try {
      this.eventSource = new ES(this.streamUrl);
    } catch (err) {
      this.handleConnectionError(err);
      return;
    }

    this.eventSource.onopen = () => {
      this.currentBackoffMs = this.initialBackoffMs;
    };

    const handlePayload = (ev: MessageEvent) => {
      try {
        if (!ev.data) return;
        const raw = JSON.parse(ev.data);
        const status = normalizeCaffeineStatus(raw);
        this.onStatusCallback?.(status);
      } catch (err) {
        this.onErrorCallback?.(err);
      }
    };

    this.eventSource.onmessage = handlePayload;

    // Go daemon sends "event: state_change" and "event: heartbeat"
    this.eventSource.addEventListener('state_change', handlePayload as EventListener);
    this.eventSource.addEventListener('heartbeat', handlePayload as EventListener);
    this.eventSource.addEventListener('circuit_breaker', handlePayload as EventListener);

    this.eventSource.onerror = (err) => {
      this.handleConnectionError(err);
    };
  }

  private handleConnectionError(err: any): void {
    this.onErrorCallback?.(err);

    if (this.isManuallyClosed) {
      return;
    }

    this.cleanupEventSource();
    this.scheduleReconnect();
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer !== null || this.isManuallyClosed) {
      return;
    }

    const delay = this.currentBackoffMs;
    this.currentBackoffMs = Math.min(
      Math.round(this.currentBackoffMs * this.backoffMultiplier),
      this.maxBackoffMs
    );

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (!this.isManuallyClosed) {
        this.initiateConnection();
      }
    }, delay);
  }

  private cleanupEventSource(): void {
    if (this.eventSource) {
      try {
        this.eventSource.close();
      } catch {
        // ignore close error
      }
      this.eventSource = null;
    }
  }

  public disconnect(): void {
    this.isManuallyClosed = true;

    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    this.cleanupEventSource();
  }
}
