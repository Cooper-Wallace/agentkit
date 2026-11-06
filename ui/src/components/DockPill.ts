import type { CaffeineStatusResponse, CaffeineState } from '../types/state.js';
import { PulseRing } from './PulseRing.js';
import { ExpandedCard, type ExpandedCardActions } from './ExpandedCard.js';

export type DockPosition = 'pos-top-center' | 'pos-top-right' | 'pos-bottom-right';

export interface DockPillOptions {
  position?: DockPosition;
  onAction?: {
    onForceAwake?: () => void;
    onCancel?: () => void;
    onDismiss?: () => void;
    onSleep?: () => void;
  };
}

const POSITIONS: DockPosition[] = ['pos-top-center', 'pos-top-right', 'pos-bottom-right'];
const STATE_CLASSES: CaffeineState[] = ['active', 'cooldown', 'override', 'idle', 'approval'];

export class DockPill {
  public element: HTMLElement;
  public pillElement: HTMLElement;
  public pulseRing: PulseRing;
  public expandedCard: ExpandedCard;
  public position: DockPosition;

  public agentTagElement: HTMLElement;
  public stateLabelElement: HTMLElement;
  public cooldownIndicatorElement: HTMLElement;
  public statusBadgeElement: HTMLElement;
  public currentStatus?: CaffeineStatusResponse;

  constructor(options?: DockPillOptions) {
    this.position = options?.position ?? 'pos-top-center';
    this.pulseRing = new PulseRing('idle');
    this.expandedCard = new ExpandedCard(options?.onAction as ExpandedCardActions);

    // Outer container: .hud-pill-container
    this.element = document.createElement('div');
    this.element.className = `hud-pill-container ${this.position} state-idle`;

    // Inner pill: .hud-pill
    this.pillElement = document.createElement('div');
    this.pillElement.className = 'hud-pill';

    // Sub-elements in pill
    this.agentTagElement = document.createElement('span');
    this.agentTagElement.className = 'agent-tag';
    this.agentTagElement.textContent = 'AGENTKIT';

    const contentWrapper = document.createElement('div');
    contentWrapper.className = 'hud-content';

    const titleRow = document.createElement('div');
    titleRow.className = 'hud-title-row';

    this.stateLabelElement = document.createElement('span');
    this.stateLabelElement.className = 'hud-status-text';
    this.stateLabelElement.textContent = 'IDLE';

    this.cooldownIndicatorElement = document.createElement('span');
    this.cooldownIndicatorElement.className = 'cooldown-indicator';
    this.cooldownIndicatorElement.style.display = 'none';

    titleRow.appendChild(this.stateLabelElement);
    titleRow.appendChild(this.cooldownIndicatorElement);
    contentWrapper.appendChild(titleRow);

    this.statusBadgeElement = document.createElement('div');
    this.statusBadgeElement.className = 'lock-badge';
    this.statusBadgeElement.textContent = 'IDLE';

    // Assemble pill
    this.pillElement.appendChild(this.pulseRing.render());
    this.pillElement.appendChild(this.agentTagElement);
    this.pillElement.appendChild(contentWrapper);
    this.pillElement.appendChild(this.statusBadgeElement);

    // Assemble container
    this.element.appendChild(this.pillElement);
    this.element.appendChild(this.expandedCard.element);

    // Clicking the pill toggles expandedCard
    this.pillElement.addEventListener('click', (e) => {
      e?.stopPropagation?.();
      this.expandedCard.toggle();
    });
  }

  public setPosition(pos: DockPosition): void {
    for (const p of POSITIONS) {
      this.element.classList.remove(p);
    }
    this.position = pos;
    this.element.classList.add(pos);
  }

  public render(status: CaffeineStatusResponse): HTMLElement {
    this.update(status);
    return this.element;
  }

  public update(status: CaffeineStatusResponse): void {
    this.currentStatus = status;

    // 1. Update outer container state classes
    for (const s of STATE_CLASSES) {
      this.element.classList.remove(`state-${s}`);
    }
    this.element.classList.add(`state-${status.state}`);

    // Ensure position class remains
    if (!this.element.classList.contains(this.position)) {
      this.element.classList.add(this.position);
    }

    // 2. Update Pulse Ring
    this.pulseRing.update(status.state, this.element);

    // 3. Update Active Subagent Badge
    let badgeText = 'AGENTKIT';
    if (status.active_leases && status.active_leases.length > 0) {
      const first = status.active_leases[0];
      badgeText = first.model || first.session_id || 'AGY';
    }
    this.agentTagElement.textContent = badgeText;

    // 4. Update State Label
    this.stateLabelElement.textContent = status.state.toUpperCase();

    // 5. Update Cooldown Countdown Indicator
    if (status.state === 'cooldown') {
      this.cooldownIndicatorElement.textContent = `${status.cooldown_remaining_seconds}s`;
      this.cooldownIndicatorElement.style.display = 'inline';
    } else {
      this.cooldownIndicatorElement.style.display = 'none';
      this.cooldownIndicatorElement.textContent = '';
    }

    // 6. Update Status Badge (SLEEP LOCKED, COOLDOWN 45s, MANUAL LOCK, IDLE)
    let badgeLabel = 'IDLE';
    switch (status.state) {
      case 'active':
        badgeLabel = status.lock_held ? 'SLEEP LOCKED' : 'ACTIVE';
        break;
      case 'cooldown':
        badgeLabel = `COOLDOWN ${status.cooldown_remaining_seconds}s`;
        break;
      case 'override':
        badgeLabel = 'MANUAL LOCK';
        break;
      case 'approval':
        badgeLabel = 'WAITING APPROVAL';
        break;
      case 'idle':
      default:
        badgeLabel = 'IDLE';
        break;
    }
    this.statusBadgeElement.textContent = badgeLabel;

    // 7. Update Expanded Card
    this.expandedCard.update(status);
  }
}
