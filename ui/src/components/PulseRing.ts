import type { CaffeineState } from '../types/state.js';

const STATE_CLASSES: CaffeineState[] = ['active', 'cooldown', 'override', 'idle', 'approval'];

export class PulseRing {
  public element: HTMLElement;
  public state: CaffeineState;

  constructor(initialState: CaffeineState = 'idle') {
    this.state = initialState;
    this.element = document.createElement('div');
    this.element.className = 'pulse-ring';
    this.update(initialState);
  }

  public render(): HTMLElement {
    return this.element;
  }

  public update(state: CaffeineState, container?: HTMLElement): void {
    this.state = state;

    for (const s of STATE_CLASSES) {
      this.element.classList.remove(`state-${s}`);
    }
    this.element.classList.add('pulse-ring');
    this.element.classList.add(`state-${state}`);
    this.element.setAttribute('data-state', state);

    const targetContainer = container ?? (this.element.parentElement as HTMLElement | null);
    if (targetContainer) {
      for (const s of STATE_CLASSES) {
        targetContainer.classList.remove(`state-${s}`);
      }
      targetContainer.classList.add(`state-${state}`);
    }
  }
}
