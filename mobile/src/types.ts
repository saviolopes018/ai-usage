export type UsageWindow = {
  usedPercentage: number;
  remainingPercentage: number;
  resetsAt: string;
};

export type ProviderUsage = {
  provider: 'codex' | 'claude' | string;
  available: boolean;
  observedAt: string;
  fiveHour?: UsageWindow;
  weekly?: UsageWindow;
  tokens?: TokenUsage;
};

export type TokenUsage = {
  inputTokens: number;
  outputTokens: number;
  cachedInputTokens: number;
  totalTokens: number;
  periods?: Record<string, TokenPeriodUsage>;
};

export type TokenPeriodUsage = Omit<TokenUsage, 'periods'>;

export type UsageSnapshot = {
  protocolVersion: number;
  agentVersion: string;
  capabilities: string[];
  device: string;
  online: boolean;
  updatedAt: string;
  providers: ProviderUsage[];
};

export type ConnectionProfile = {
  mode: 'local' | 'remote';
  endpoint: string;
  token: string;
  deviceId?: string;
  deviceName?: string;
  refreshIntervalMs?: 30_000 | 60_000 | 300_000;
};

export type PairedDevice = { id:string; name:string; createdAt:string; lastSeen?:string };
export type AuthInfo = {kind:'master'|'device';migrationRequired:boolean;credentialId?:string};

export type PairingPayload = {
  type: 'ai-usage-monitor-pairing';
  version: 2;
  endpoint: string;
  ticket: string;
  device: string;
  deviceId: string;
  expiresAt: string;
};

export type DiscoveredDevice = {
  id: string;
  name: string;
  endpoint: string;
};

export type ConnectionState = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'offline';

export type NotificationPreferences = {
  enabled: boolean;
  thresholds: number[];
  providerUnavailable: boolean;
  staleData: boolean;
  windowReset: boolean;
  fiveHourAlerts: boolean;
  weeklyAlerts: boolean;
  predictiveAlerts: boolean;
  quietHours: boolean;
  quietStartHour: number;
  quietEndHour: number;
};
