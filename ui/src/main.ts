import './styles/glassmorphism.css';
import { CaffeineClient } from './api/client.js';
import { CaffeineEventStream } from './api/stream.js';
import type { CaffeineStatusResponse } from './types/state.js';

export { CaffeineClient, CaffeineEventStream };
export type * from './types/state.js';

function renderApp(status: CaffeineStatusResponse) {
  const app = document.getElementById('app');
  if (!app) return;

  const stateClass = `state-${status.state}`;
  app.innerHTML = `
    <div class="hud-pill ${stateClass}">
      <span class="pulse-ring"></span>
      <span style="font-weight: 600; text-transform: uppercase; font-size: 13px;">${status.state}</span>
      ${
        status.active_leases.length > 0
          ? status.active_leases
              .map((l) => `<span class="agent-tag">${l.model || l.session_id}</span>`)
              .join(' ')
          : '<span style="color: var(--text-muted); font-size: 12px;">idle</span>'
      }
    </div>
  `;
}

// In browser runtime, initialize client & event stream
if (typeof document !== 'undefined') {
  const client = new CaffeineClient();
  const stream = new CaffeineEventStream();

  client
    .getStatus()
    .then(renderApp)
    .catch((err) => {
      console.warn('Caffeine daemon initial fetch:', err.message);
      renderApp({
        state: 'idle',
        active_inhibitor: 'none',
        lock_held: false,
        cooldown_remaining_seconds: 0,
        override_remaining_seconds: 0,
        active_leases: [],
      });
    });

  stream.connect(
    (status) => renderApp(status),
    (err) => console.debug('Caffeine stream reconnecting...', err)
  );
}
