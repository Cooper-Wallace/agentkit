import type { CaffeineState } from '../types/state.js';

export interface ControlsCallbacks {
  onForceAwake?: () => void;
  onCancel?: () => void;
  onDismiss?: () => void;
  onSleep?: () => void;
}

export class Controls {
  public element: HTMLElement;
  public onForceAwake?: () => void;
  public onCancel?: () => void;
  public onDismiss?: () => void;
  public onSleep?: () => void;

  public forceAwakeBtn: HTMLButtonElement;
  public dismissBtn: HTMLButtonElement;
  public sleepBtn: HTMLButtonElement;
  public cancelBtn: HTMLButtonElement;
  public currentState: CaffeineState = 'idle';

  constructor(callbacks: ControlsCallbacks = {}) {
    this.onForceAwake = callbacks.onForceAwake;
    this.onCancel = callbacks.onCancel;
    this.onDismiss = callbacks.onDismiss;
    this.onSleep = callbacks.onSleep;

    this.element = document.createElement('div');
    this.element.className = 'card-actions hud-controls';

    // Force Awake Button (accent-blue)
    this.forceAwakeBtn = document.createElement('button') as HTMLButtonElement;
    this.forceAwakeBtn.className = 'action-btn btn-force-awake';
    this.forceAwakeBtn.textContent = 'Force Awake';
    this.forceAwakeBtn.style.color = 'var(--accent-blue, #2979ff)';
    this.forceAwakeBtn.style.borderColor = 'rgba(41, 121, 255, 0.4)';
    this.forceAwakeBtn.addEventListener('click', (e) => {
      e?.stopPropagation?.();
      this.onForceAwake?.();
    });

    // Dismiss to Idle Button
    this.dismissBtn = document.createElement('button') as HTMLButtonElement;
    this.dismissBtn.className = 'action-btn btn-dismiss';
    this.dismissBtn.textContent = 'Dismiss to Idle';
    this.dismissBtn.addEventListener('click', (e) => {
      e?.stopPropagation?.();
      this.onDismiss?.();
    });

    // Sleep Now Button
    this.sleepBtn = document.createElement('button') as HTMLButtonElement;
    this.sleepBtn.className = 'action-btn btn-sleep';
    this.sleepBtn.textContent = 'Sleep Now';
    this.sleepBtn.addEventListener('click', (e) => {
      e?.stopPropagation?.();
      this.onSleep?.();
    });

    // Cancel Button
    this.cancelBtn = document.createElement('button') as HTMLButtonElement;
    this.cancelBtn.className = 'action-btn btn-cancel';
    this.cancelBtn.textContent = 'Cancel';
    this.cancelBtn.addEventListener('click', (e) => {
      e?.stopPropagation?.();
      this.onCancel?.();
    });

    this.element.appendChild(this.forceAwakeBtn);
    this.element.appendChild(this.dismissBtn);
    this.element.appendChild(this.sleepBtn);
    this.element.appendChild(this.cancelBtn);
  }

  public render(): HTMLElement {
    return this.element;
  }

  public updateState(state: CaffeineState): void {
    this.currentState = state;
    this.element.setAttribute('data-state', state);

    if (state === 'override') {
      this.forceAwakeBtn.classList.add('active');
    } else {
      this.forceAwakeBtn.classList.remove('active');
    }

    if (state === 'idle') {
      this.dismissBtn.setAttribute('disabled', 'true');
    } else {
      this.dismissBtn.removeAttribute('disabled');
    }
  }
}
