export type CaffeineState = 'active' | 'cooldown' | 'override' | 'idle' | 'approval';

export interface AgentLease {
  session_id: string;
  model?: string;
  started_at?: string;
  state: string;
  description?: string;
}

export interface HardwareStatus {
  battery_percent?: number;
  is_charging?: boolean;
  cpu_temp_c?: number;
  lid_closed?: boolean;
}

export interface CircuitBreakers {
  battery_tripped?: boolean;
  thermal_tripped?: boolean;
  lid_tripped?: boolean;
  ceiling_tripped?: boolean;
}

export interface StateTransitionEvent {
  timestamp: string;
  from: CaffeineState;
  to: CaffeineState;
  reason: string;
}

export interface CaffeineStatusResponse {
  state: CaffeineState;
  active_inhibitor: string;
  lock_held: boolean;
  cooldown_remaining_seconds: number;
  override_remaining_seconds: number;
  active_leases: AgentLease[];
  hardware?: HardwareStatus;
  circuit_breakers?: CircuitBreakers;
  history?: StateTransitionEvent[];
}
