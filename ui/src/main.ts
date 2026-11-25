import './styles/glassmorphism.css';
import { CaffeineClient } from './api/client.js';
import { CaffeineEventStream } from './api/stream.js';
import { DockPill, type DockPosition } from './components/DockPill.js';
import type { CaffeineStatusResponse } from './types/state.js';

export { DockPill, CaffeineClient, CaffeineEventStream };
export type * from './types/state.js';
export type { DockPosition } from './components/DockPill.js';

export interface MountCaffeineDockOptions {
  baseUrl?: string;
  position?: DockPosition;
  autoConnect?: boolean;
}

export interface MountCaffeineDockResult {
  pill: DockPill;
  client: CaffeineClient;
  stream: CaffeineEventStream;
  destroy: () => void;
}

const FALLBACK_IDLE_STATUS: CaffeineStatusResponse = {
  state: 'idle',
  active_inhibitor: 'none',
  lock_held: false,
  cooldown_remaining_seconds: 0,
  override_remaining_seconds: 0,
  active_leases: [],
};

export function mountCaffeineDock(
  container: HTMLElement,
  options?: MountCaffeineDockOptions
): MountCaffeineDockResult {
  const baseUrl = options?.baseUrl;
  const position = options?.position ?? 'pos-top-center';
  const autoConnect = options?.autoConnect !== false;

  const client = new CaffeineClient({ baseUrl });
  const stream = new CaffeineEventStream({ baseUrl });

  const refreshStatus = () => {
    client
      .getStatus()
      .then((status) => pill.update(status))
      .catch((err) => {
        console.debug('Failed to refresh status:', err);
      });
  };

  const pill = new DockPill({
    position,
    onAction: {
      onForceAwake: () => {
        client
          .override()
          .then(refreshStatus)
          .catch((err) => {
            console.debug('onForceAwake error:', err);
          });
      },
      onCancel: () => {
        client
          .cancel()
          .then(refreshStatus)
          .catch((err) => {
            console.debug('onCancel error:', err);
          });
      },
      onDismiss: () => {
        client
          .notify('user-dismiss', 'idle')
          .then(refreshStatus)
          .catch((err) => {
            console.debug('onDismiss error:', err);
          });
      },
      onSleep: () => {
        client
          .override('0s')
          .then(refreshStatus)
          .catch((err) => {
            console.debug('onSleep error:', err);
          });
      },
    },
  });

  // Mount into container
  container.appendChild(pill.element);

  const isDescendant = (parent: HTMLElement, target: any): boolean => {
    if (!parent || !target) return false;
    if (typeof parent.contains === 'function') {
      return parent.contains(target);
    }
    let current = target;
    while (current) {
      if (current === parent) return true;
      current = current.parentElement;
    }
    return false;
  };

  const handleDocumentClick = (event: any) => {
    if (!pill.expandedCard.isOpen) return;
    const target = event?.target;
    if (target && !isDescendant(pill.element, target)) {
      pill.expandedCard.toggle(false);
    }
  };

  const handleKeyDown = (event: any) => {
    if (event?.key === 'Escape' || event?.key === 'Esc') {
      if (pill.expandedCard.isOpen) {
        pill.expandedCard.toggle(false);
      }
    }
  };

  if (typeof document !== 'undefined') {
    document.addEventListener('click', handleDocumentClick);
    document.addEventListener('keydown', handleKeyDown);
  }

  if (autoConnect) {
    client
      .getStatus()
      .then((status) => pill.update(status))
      .catch((err) => {
        console.debug('Caffeine daemon offline, falling back to idle:', err);
        pill.update(FALLBACK_IDLE_STATUS);
      });

    stream.connect(
      (status) => pill.update(status),
      (err) => console.debug('Caffeine stream reconnecting...', err)
    );
  }

  const destroy = () => {
    stream.disconnect();
    if (typeof document !== 'undefined') {
      document.removeEventListener('click', handleDocumentClick);
      document.removeEventListener('keydown', handleKeyDown);
    }
    if (typeof pill.element.remove === 'function') {
      pill.element.remove();
    } else if (pill.element.parentElement) {
      pill.element.parentElement.removeChild(pill.element);
    }
  };

  return {
    pill,
    client,
    stream,
    destroy,
  };
}

export default mountCaffeineDock;

// Auto-mount in browser if document and #app exist
if (typeof document !== 'undefined') {
  const appContainer = document.getElementById('app');
  if (appContainer) {
    const dock = mountCaffeineDock(appContainer);
    if (typeof window !== 'undefined') {
      (window as any).__caffeineDock = dock;
    }
  }
}
