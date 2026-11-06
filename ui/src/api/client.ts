import type {
  CaffeineState,
  CaffeineStatusResponse,
  StateTransitionEvent,
  AgentLease,
} from '../types/state.js';

export class CaffeineAPIError extends Error {
  public readonly status?: number;
  public readonly endpoint: string;
  public readonly details?: string;

  constructor(message: string, endpoint: string, status?: number, details?: string) {
    super(message);
    this.name = 'CaffeineAPIError';
    this.endpoint = endpoint;
    this.status = status;
    this.details = details;
  }
}

export interface CaffeineClientConfig {
  baseUrl?: string;
  fetchFn?: typeof fetch;
}

export function normalizeState(rawState: unknown): CaffeineState {
  if (typeof rawState !== 'string') return 'idle';
  const s = rawState.trim().toLowerCase();
  if (s === 'active') return 'active';
  if (s === 'cooldown') return 'cooldown';
  if (s === 'override') return 'override';
  if (s === 'idle') return 'idle';
  if (s === 'approval' || s === 'waiting_approval' || s === 'waiting-approval') return 'approval';
  return 'idle';
}

export function normalizeCaffeineStatus(raw: any): CaffeineStatusResponse {
  if (!raw || typeof raw !== 'object') {
    return {
      state: 'idle',
      active_inhibitor: 'none',
      lock_held: false,
      cooldown_remaining_seconds: 0,
      override_remaining_seconds: 0,
      active_leases: [],
    };
  }

  const rawState = raw.state;
  const state: CaffeineState = normalizeState(rawState);

  const lock_held = Boolean(raw.lock_held ?? raw.inhibited ?? false);
  const active_inhibitor = raw.active_inhibitor ?? (lock_held ? 'systemd-inhibit' : 'none');
  const cooldown_remaining_seconds = Number(raw.cooldown_remaining_seconds ?? raw.cooldown_remaining ?? 0);
  const override_remaining_seconds = Number(raw.override_remaining_seconds ?? raw.override_remaining ?? 0);

  let active_leases: AgentLease[] = [];
  if (Array.isArray(raw.active_leases)) {
    active_leases = raw.active_leases.map((lease: any) => ({
      session_id: String(lease.session_id || ''),
      model: lease.model ?? lease.agent_name,
      started_at: lease.started_at ?? lease.start_time,
      state: String(lease.state || ''),
      description: lease.description ?? lease.task,
    }));
  } else if (Array.isArray(raw.leases)) {
    active_leases = raw.leases.map((lease: any) => ({
      session_id: String(lease.session_id || ''),
      model: lease.agent_name ?? lease.model,
      started_at: lease.start_time ?? lease.started_at,
      state: String(lease.state || ''),
      description: lease.task ?? lease.description,
    }));
  }

  const hardware = raw.hardware
    ? {
        battery_percent: raw.hardware.battery_percent !== undefined ? Number(raw.hardware.battery_percent) : undefined,
        is_charging: raw.hardware.is_charging ?? raw.hardware.battery_charging,
        cpu_temp_c: raw.hardware.cpu_temp_c !== undefined ? Number(raw.hardware.cpu_temp_c) : undefined,
        lid_closed: raw.hardware.lid_closed !== undefined ? Boolean(raw.hardware.lid_closed) : undefined,
      }
    : undefined;

  const circuit_breakers = raw.circuit_breakers
    ? {
        battery_tripped: raw.circuit_breakers.battery_tripped !== undefined ? Boolean(raw.circuit_breakers.battery_tripped) : undefined,
        thermal_tripped: raw.circuit_breakers.thermal_tripped !== undefined ? Boolean(raw.circuit_breakers.thermal_tripped) : undefined,
        lid_tripped: raw.circuit_breakers.lid_tripped !== undefined ? Boolean(raw.circuit_breakers.lid_tripped) : undefined,
        ceiling_tripped: raw.circuit_breakers.ceiling_tripped !== undefined ? Boolean(raw.circuit_breakers.ceiling_tripped) : undefined,
      }
    : undefined;

  const history = Array.isArray(raw.history)
    ? raw.history.map((h: any) => ({
        timestamp: String(h.timestamp || ''),
        from: normalizeState(h.from ?? h.from_state),
        to: normalizeState(h.to ?? h.to_state),
        reason: String(h.reason || ''),
      }))
    : undefined;

  return {
    state,
    active_inhibitor,
    lock_held,
    cooldown_remaining_seconds,
    override_remaining_seconds,
    active_leases,
    hardware,
    circuit_breakers,
    history,
  };
}

export class CaffeineClient {
  private readonly baseUrl: string;
  private readonly fetchFn: typeof fetch;

  constructor(config: CaffeineClientConfig = {}) {
    this.baseUrl = (config.baseUrl ?? 'http://127.0.0.1:18820').replace(/\/+$/, '');
    this.fetchFn = config.fetchFn ?? globalThis.fetch?.bind(globalThis);
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    let response: Response;
    try {
      response = await this.fetchFn(url, {
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
          ...options.headers,
        },
        ...options,
      });
    } catch (err: any) {
      throw new CaffeineAPIError(
        `Network error connecting to caffeine daemon at ${url}: ${err.message || String(err)}`,
        endpoint,
        undefined,
        err.message || String(err)
      );
    }

    if (!response.ok) {
      let errorBody = '';
      try {
        errorBody = await response.text();
      } catch {
        // ignore body read error
      }
      throw new CaffeineAPIError(
        `Caffeine daemon error on ${endpoint}: ${response.status} ${response.statusText}${errorBody ? ` - ${errorBody}` : ''}`,
        endpoint,
        response.status,
        errorBody
      );
    }

    try {
      return (await response.json()) as T;
    } catch (err: any) {
      throw new CaffeineAPIError(
        `Failed to parse JSON response from ${endpoint}: ${err.message || String(err)}`,
        endpoint,
        response.status,
        err.message || String(err)
      );
    }
  }

  async getStatus(): Promise<CaffeineStatusResponse> {
    const raw = await this.request<any>('/status', { method: 'GET' });
    return normalizeCaffeineStatus(raw);
  }

  async notify(sessionId: string, state: string, description?: string): Promise<boolean> {
    if (!sessionId || !sessionId.trim()) {
      throw new CaffeineAPIError('sessionId is required for notify', '/notify', 400);
    }
    if (!state || !state.trim()) {
      throw new CaffeineAPIError('state is required for notify', '/notify', 400);
    }

    await this.request('/notify', {
      method: 'POST',
      body: JSON.stringify({
        session_id: sessionId.trim(),
        state: state.trim(),
        task: description ?? '',
      }),
    });
    return true;
  }

  async override(duration?: string): Promise<boolean> {
    await this.request('/override', {
      method: 'POST',
      body: JSON.stringify({
        enabled: true,
        duration: duration ? duration.trim() : '',
      }),
    });
    return true;
  }

  async cancel(): Promise<boolean> {
    await this.request('/cancel', {
      method: 'POST',
    });
    return true;
  }

  async getHistory(): Promise<StateTransitionEvent[]> {
    const raw = await this.request<any[]>('/history', { method: 'GET' });
    if (!Array.isArray(raw)) {
      return [];
    }
    return raw.map((entry) => ({
      timestamp: String(entry.timestamp || ''),
      from: normalizeState(entry.from ?? entry.from_state),
      to: normalizeState(entry.to ?? entry.to_state),
      reason: String(entry.reason || ''),
    }));
  }
}
