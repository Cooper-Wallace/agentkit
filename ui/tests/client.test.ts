import { describe, it, expect, vi, beforeEach } from 'vitest';
import { CaffeineClient, CaffeineAPIError, normalizeState, normalizeCaffeineStatus } from '../src/api/client.js';

describe('CaffeineClient', () => {
  let mockFetch: ReturnType<typeof vi.fn>;
  let client: CaffeineClient;

  beforeEach(() => {
    mockFetch = vi.fn();
    client = new CaffeineClient({
      baseUrl: 'http://127.0.0.1:18820',
      fetchFn: mockFetch as unknown as typeof fetch,
    });
  });

  describe('normalizeState', () => {
    it('normalizes uppercase and lowercase state strings', () => {
      expect(normalizeState('ACTIVE')).toBe('active');
      expect(normalizeState('active')).toBe('active');
      expect(normalizeState('COOLDOWN')).toBe('cooldown');
      expect(normalizeState('OVERRIDE')).toBe('override');
      expect(normalizeState('IDLE')).toBe('idle');
      expect(normalizeState('WAITING_APPROVAL')).toBe('approval');
      expect(normalizeState('approval')).toBe('approval');
      expect(normalizeState('unknown-state')).toBe('idle');
      expect(normalizeState(null)).toBe('idle');
    });
  });

  describe('normalizeCaffeineStatus', () => {
    it('normalizes raw Go engine response', () => {
      const raw = {
        state: 'ACTIVE',
        inhibited: true,
        leases: [
          {
            session_id: 'session-123',
            agent_name: 'gemini-flash',
            task: 'unit-testing',
            state: 'ACTIVE',
            start_time: '2026-11-06T12:00:00Z',
          },
        ],
        cooldown_remaining: 15.5,
        override_remaining: 0,
        hardware: {
          battery_percent: 88,
          battery_charging: true,
          cpu_temp_c: 48.5,
          lid_closed: false,
        },
        circuit_breakers: {
          tripped: false,
          battery_tripped: false,
          thermal_tripped: false,
          lid_tripped: false,
          ceiling_tripped: false,
        },
      };

      const normalized = normalizeCaffeineStatus(raw);
      expect(normalized.state).toBe('active');
      expect(normalized.lock_held).toBe(true);
      expect(normalized.active_inhibitor).toBe('systemd-inhibit');
      expect(normalized.cooldown_remaining_seconds).toBe(15.5);
      expect(normalized.active_leases).toHaveLength(1);
      expect(normalized.active_leases[0]).toEqual({
        session_id: 'session-123',
        model: 'gemini-flash',
        started_at: '2026-11-06T12:00:00Z',
        state: 'ACTIVE',
        description: 'unit-testing',
      });
      expect(normalized.hardware).toEqual({
        battery_percent: 88,
        is_charging: true,
        cpu_temp_c: 48.5,
        lid_closed: false,
      });
      expect(normalized.circuit_breakers).toEqual({
        battery_tripped: false,
        thermal_tripped: false,
        lid_tripped: false,
        ceiling_tripped: false,
      });
    });

    it('handles empty or malformed input safely', () => {
      const normalized = normalizeCaffeineStatus(null);
      expect(normalized.state).toBe('idle');
      expect(normalized.lock_held).toBe(false);
      expect(normalized.active_leases).toEqual([]);
    });
  });

  describe('getStatus', () => {
    it('fetches and returns normalized status on success', async () => {
      const mockStatus = {
        state: 'IDLE',
        inhibited: false,
        leases: [],
        cooldown_remaining: 0,
        override_remaining: 0,
      };

      mockFetch.mockResolvedValueOnce(
        new Response(JSON.stringify(mockStatus), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      );

      const status = await client.getStatus();
      expect(mockFetch).toHaveBeenCalledWith(
        'http://127.0.0.1:18820/status',
        expect.objectContaining({ method: 'GET' })
      );
      expect(status.state).toBe('idle');
      expect(status.lock_held).toBe(false);
      expect(status.active_leases).toEqual([]);
    });
  });

  describe('notify', () => {
    it('sends POST to /notify with lease payload and returns true', async () => {
      mockFetch.mockResolvedValueOnce(
        new Response(JSON.stringify({ status: 'OK' }), { status: 200 })
      );

      const result = await client.notify('test-session', 'active', 'background processing');
      expect(result).toBe(true);
      expect(mockFetch).toHaveBeenCalledWith(
        'http://127.0.0.1:18820/notify',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            session_id: 'test-session',
            state: 'active',
            task: 'background processing',
          }),
        })
      );
    });

    it('rejects with error when sessionId or state is missing', async () => {
      await expect(client.notify('', 'active')).rejects.toThrow(CaffeineAPIError);
      await expect(client.notify('session', '')).rejects.toThrow(CaffeineAPIError);
    });
  });

  describe('override and cancel', () => {
    it('sends POST to /override with duration', async () => {
      mockFetch.mockResolvedValueOnce(
        new Response(JSON.stringify({ status: 'OK' }), { status: 200 })
      );

      const result = await client.override('30m');
      expect(result).toBe(true);
      expect(mockFetch).toHaveBeenCalledWith(
        'http://127.0.0.1:18820/override',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ enabled: true, duration: '30m' }),
        })
      );
    });

    it('sends POST to /cancel and returns true', async () => {
      mockFetch.mockResolvedValueOnce(
        new Response(JSON.stringify({ status: 'OK' }), { status: 200 })
      );

      const result = await client.cancel();
      expect(result).toBe(true);
      expect(mockFetch).toHaveBeenCalledWith(
        'http://127.0.0.1:18820/cancel',
        expect.objectContaining({ method: 'POST' })
      );
    });
  });

  describe('getHistory', () => {
    it('fetches and maps transition history', async () => {
      const historyRaw = [
        {
          timestamp: '2026-11-06T10:00:00Z',
          from_state: 'IDLE',
          to_state: 'ACTIVE',
          reason: 'session lease started',
        },
      ];

      mockFetch.mockResolvedValueOnce(
        new Response(JSON.stringify(historyRaw), { status: 200 })
      );

      const history = await client.getHistory();
      expect(history).toEqual([
        {
          timestamp: '2026-11-06T10:00:00Z',
          from: 'idle',
          to: 'active',
          reason: 'session lease started',
        },
      ]);
    });
  });

  describe('Guarded Failure Handling', () => {
    it('handles daemon 500 Internal Server Error cleanly', async () => {
      mockFetch.mockResolvedValueOnce(
        new Response('internal error: inhibitor failure', {
          status: 500,
          statusText: 'Internal Server Error',
        })
      );

      let caughtError: any;
      try {
        await client.getStatus();
      } catch (err: any) {
        caughtError = err;
      }

      expect(caughtError).toBeInstanceOf(CaffeineAPIError);
      expect(caughtError.status).toBe(500);
      expect(caughtError.endpoint).toBe('/status');
      expect(caughtError.message).toContain('500');
      expect(caughtError.details).toContain('internal error: inhibitor failure');
    });

    it('handles daemon unreachable / network connection refusal cleanly', async () => {
      mockFetch.mockRejectedValueOnce(new Error('connect ECONNREFUSED 127.0.0.1:18820'));

      let caughtError: any;
      try {
        await client.getStatus();
      } catch (err: any) {
        caughtError = err;
      }

      expect(caughtError).toBeInstanceOf(CaffeineAPIError);
      expect(caughtError.status).toBeUndefined();
      expect(caughtError.message).toContain('Network error connecting to caffeine daemon');
      expect(caughtError.message).toContain('ECONNREFUSED');
    });

    it('handles invalid JSON responses gracefully', async () => {
      mockFetch.mockResolvedValueOnce(
        new Response('<html>Bad Gateway</html>', {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      );

      let caughtError: any;
      try {
        await client.getStatus();
      } catch (err: any) {
        caughtError = err;
      }

      expect(caughtError).toBeInstanceOf(CaffeineAPIError);
      expect(caughtError.message).toContain('Failed to parse JSON response');
    });
  });
});
