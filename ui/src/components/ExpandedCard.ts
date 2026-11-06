import type { CaffeineStatusResponse } from '../types/state.js';
import { Controls, type ControlsCallbacks } from './Controls.js';

export interface ExpandedCardActions extends ControlsCallbacks {
  onToggle?: (open: boolean) => void;
}

export class ExpandedCard {
  public element: HTMLElement;
  public controls: Controls;
  public isOpen: boolean;
  public onAction: ExpandedCardActions;

  constructor(onAction: ExpandedCardActions = {}) {
    this.onAction = onAction;
    this.controls = new Controls(onAction);
    this.isOpen = false;
    this.element = document.createElement('div');
    this.element.className = 'expanded-card hud-expanded-card';
    this.element.style.display = 'none';
  }

  public toggle(force?: boolean): boolean {
    if (typeof force === 'boolean') {
      this.isOpen = force;
    } else {
      this.isOpen = !this.isOpen;
    }

    if (this.isOpen) {
      this.element.classList.add('visible', 'open');
      this.element.style.display = 'flex';
    } else {
      this.element.classList.remove('visible', 'open');
      this.element.style.display = 'none';
    }

    this.onAction.onToggle?.(this.isOpen);
    return this.isOpen;
  }

  public render(status: CaffeineStatusResponse): HTMLElement {
    this.update(status);
    return this.element;
  }

  public update(status: CaffeineStatusResponse): void {
    // Clear element
    while (this.element.firstChild) {
      this.element.removeChild(this.element.firstChild);
    }

    // Maintain open/closed display style
    this.element.style.display = this.isOpen ? 'flex' : 'none';
    if (this.isOpen) {
      this.element.classList.add('visible', 'open');
    } else {
      this.element.classList.remove('visible', 'open');
    }

    // Header
    const header = document.createElement('div');
    header.className = 'expanded-card-header';
    const title = document.createElement('span');
    title.className = 'expanded-card-title';
    title.textContent = 'Caffeine Power Guard';
    header.appendChild(title);
    this.element.appendChild(header);

    // Section 1: Active Leases
    const leasesSection = document.createElement('div');
    leasesSection.className = 'expanded-card-section leases-section';
    const leasesTitle = document.createElement('span');
    leasesTitle.className = 'expanded-card-section-title';
    leasesTitle.textContent = 'Active Leases';
    leasesSection.appendChild(leasesTitle);

    if (status.active_leases && status.active_leases.length > 0) {
      const leasesList = document.createElement('div');
      leasesList.className = 'leases-list';
      for (const lease of status.active_leases) {
        const item = document.createElement('div');
        item.className = 'card-row lease-item';

        const badge = document.createElement('span');
        badge.className = 'agent-tag lease-badge';
        badge.textContent = lease.model || lease.session_id;

        const info = document.createElement('span');
        info.className = 'lease-status';
        let detail = lease.state;
        if (lease.started_at) detail += ` · ${lease.started_at}`;
        if (lease.description) detail += ` (${lease.description})`;
        info.textContent = detail;

        item.appendChild(badge);
        item.appendChild(info);
        leasesList.appendChild(item);
      }
      leasesSection.appendChild(leasesList);
    } else {
      const empty = document.createElement('span');
      empty.className = 'empty-leases card-row';
      empty.style.color = 'var(--text-muted, #8b9bb4)';
      empty.textContent = 'No active leases (idle)';
      leasesSection.appendChild(empty);
    }
    this.element.appendChild(leasesSection);

    // Section 2: System / Inhibition
    const systemSection = document.createElement('div');
    systemSection.className = 'expanded-card-section system-section';

    // Inhibition Backend
    const inhibitorRow = document.createElement('div');
    inhibitorRow.className = 'card-row inhibitor-row';
    const inhibitorLabel = document.createElement('span');
    inhibitorLabel.textContent = 'Inhibition Backend:';
    const inhibitorVal = document.createElement('span');
    inhibitorVal.className = 'inhibitor-value';
    inhibitorVal.textContent = status.active_inhibitor || 'none';
    inhibitorRow.appendChild(inhibitorLabel);
    inhibitorRow.appendChild(inhibitorVal);
    systemSection.appendChild(inhibitorRow);

    // Lock Held Status
    const lockRow = document.createElement('div');
    lockRow.className = 'card-row lock-row';
    const lockLabel = document.createElement('span');
    lockLabel.textContent = 'Sleep Lock:';
    const lockVal = document.createElement('span');
    lockVal.className = 'lock-held-value';
    lockVal.textContent = status.lock_held ? 'LOCKED' : 'UNLOCKED';
    lockRow.appendChild(lockLabel);
    lockRow.appendChild(lockVal);
    systemSection.appendChild(lockRow);

    this.element.appendChild(systemSection);

    // Section 3: Hardware Metrics (if present)
    if (status.hardware) {
      const hwSection = document.createElement('div');
      hwSection.className = 'expanded-card-section hardware-section';
      const hwTitle = document.createElement('span');
      hwTitle.className = 'expanded-card-section-title';
      hwTitle.textContent = 'Hardware Telemetry';
      hwSection.appendChild(hwTitle);

      if (status.hardware.battery_percent !== undefined) {
        const batRow = document.createElement('div');
        batRow.className = 'card-row hardware-battery-row';
        const batLabel = document.createElement('span');
        batLabel.textContent = 'Battery:';
        const batVal = document.createElement('span');
        batVal.className = 'hardware-battery';
        batVal.textContent = `${status.hardware.battery_percent}%${status.hardware.is_charging ? ' (charging)' : ''}`;
        batRow.appendChild(batLabel);
        batRow.appendChild(batVal);
        hwSection.appendChild(batRow);
      }

      if (status.hardware.cpu_temp_c !== undefined) {
        const tempRow = document.createElement('div');
        tempRow.className = 'card-row hardware-temp-row';
        const tempLabel = document.createElement('span');
        tempLabel.textContent = 'CPU Temp:';
        const tempVal = document.createElement('span');
        tempVal.className = 'hardware-temp';
        tempVal.textContent = `${status.hardware.cpu_temp_c}°C`;
        tempRow.appendChild(tempLabel);
        tempRow.appendChild(tempVal);
        hwSection.appendChild(tempRow);
      }

      if (status.hardware.lid_closed !== undefined) {
        const lidRow = document.createElement('div');
        lidRow.className = 'card-row hardware-lid-row';
        const lidLabel = document.createElement('span');
        lidLabel.textContent = 'Lid Status:';
        const lidVal = document.createElement('span');
        lidVal.className = 'hardware-lid';
        lidVal.textContent = status.hardware.lid_closed ? 'Closed' : 'Open';
        lidRow.appendChild(lidLabel);
        lidRow.appendChild(lidVal);
        hwSection.appendChild(lidRow);
      }

      this.element.appendChild(hwSection);
    }

    // Section 4: Circuit Breakers (if tripped)
    if (status.circuit_breakers) {
      const cb = status.circuit_breakers;
      const hasTripped = cb.battery_tripped || cb.thermal_tripped || cb.lid_tripped || cb.ceiling_tripped;
      if (hasTripped) {
        const cbSection = document.createElement('div');
        cbSection.className = 'expanded-card-section circuit-breakers-section';
        const cbTitle = document.createElement('span');
        cbTitle.className = 'expanded-card-section-title';
        cbTitle.textContent = 'Circuit Breaker Warning';
        cbSection.appendChild(cbTitle);

        if (cb.battery_tripped) {
          const badge = document.createElement('span');
          badge.className = 'circuit-breaker-warning breaker-battery';
          badge.textContent = 'Battery Critical (≤ 10%)';
          cbSection.appendChild(badge);
        }
        if (cb.thermal_tripped) {
          const badge = document.createElement('span');
          badge.className = 'circuit-breaker-warning breaker-thermal';
          badge.textContent = 'Thermal Ceiling Tripped (> 92°C)';
          cbSection.appendChild(badge);
        }
        if (cb.lid_tripped) {
          const badge = document.createElement('span');
          badge.className = 'circuit-breaker-warning breaker-lid';
          badge.textContent = 'Laptop Lid Closed';
          cbSection.appendChild(badge);
        }
        if (cb.ceiling_tripped) {
          const badge = document.createElement('span');
          badge.className = 'circuit-breaker-warning breaker-ceiling';
          badge.textContent = 'Runaway Ceiling Tripped (4h)';
          cbSection.appendChild(badge);
        }
        this.element.appendChild(cbSection);
      }
    }

    // Section 5: Cooldown Progress (if state is cooldown or remaining seconds > 0)
    if (status.state === 'cooldown' || status.cooldown_remaining_seconds > 0) {
      const cdSection = document.createElement('div');
      cdSection.className = 'expanded-card-section cooldown-section';

      const cdRow = document.createElement('div');
      cdRow.className = 'card-row';
      const cdLabel = document.createElement('span');
      cdLabel.textContent = 'Cooldown Buffer:';
      const cdVal = document.createElement('span');
      cdVal.className = 'cooldown-countdown';
      cdVal.textContent = `${status.cooldown_remaining_seconds}s`;
      cdRow.appendChild(cdLabel);
      cdRow.appendChild(cdVal);
      cdSection.appendChild(cdRow);

      const bar = document.createElement('div');
      bar.className = 'cooldown-bar';
      const fill = document.createElement('div');
      fill.className = 'cooldown-bar-fill';
      const pct = Math.min(100, Math.max(0, (status.cooldown_remaining_seconds / 45) * 100));
      fill.style.width = `${pct}%`;
      bar.appendChild(fill);
      cdSection.appendChild(bar);

      this.element.appendChild(cdSection);
    }

    // Embed Controls
    this.controls.updateState(status.state);
    this.element.appendChild(this.controls.render());
  }
}
