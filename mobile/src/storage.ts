import * as SecureStore from 'expo-secure-store';
import { ConnectionProfile, NotificationPreferences, UsageSnapshot } from './types';

const PROFILE_KEY = 'ai-usage-monitor.connection.v1';
const NOTIFICATIONS_KEY = 'ai-usage-monitor.notifications.v1';
const HISTORY_KEY = 'ai-usage-monitor.usage-history.v1';
export const DEFAULT_NOTIFICATION_PREFERENCES:NotificationPreferences={enabled:false,thresholds:[75,90],providerUnavailable:true,staleData:true,windowReset:true,fiveHourAlerts:true,weeklyAlerts:true,predictiveAlerts:true,quietHours:false,quietStartHour:22,quietEndHour:8};

export async function loadProfile(): Promise<ConnectionProfile | null> {
  const raw = await SecureStore.getItemAsync(PROFILE_KEY);
  if (!raw) return null;
  try { return JSON.parse(raw) as ConnectionProfile; } catch { await SecureStore.deleteItemAsync(PROFILE_KEY); return null; }
}

export async function saveProfile(profile: ConnectionProfile): Promise<void> {
  await SecureStore.setItemAsync(PROFILE_KEY, JSON.stringify(profile), { keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY });
}

export async function clearProfile(): Promise<void> {
  await SecureStore.deleteItemAsync(PROFILE_KEY);
}

export async function loadNotificationPreferences():Promise<NotificationPreferences>{
  const raw=await SecureStore.getItemAsync(NOTIFICATIONS_KEY);if(!raw)return DEFAULT_NOTIFICATION_PREFERENCES;
  try{return {...DEFAULT_NOTIFICATION_PREFERENCES,...JSON.parse(raw)} as NotificationPreferences}catch{return DEFAULT_NOTIFICATION_PREFERENCES}
}
export async function saveNotificationPreferences(value:NotificationPreferences):Promise<void>{
  await SecureStore.setItemAsync(NOTIFICATIONS_KEY,JSON.stringify(value),{keychainAccessible:SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY});
}
export async function loadUsageHistory():Promise<UsageSnapshot[]>{const raw=await SecureStore.getItemAsync(HISTORY_KEY);if(!raw)return[];try{const value=JSON.parse(raw);return Array.isArray(value)?value.slice(-12):[]}catch{return[]}}
export async function saveUsageHistory(value:UsageSnapshot[]):Promise<void>{await SecureStore.setItemAsync(HISTORY_KEY,JSON.stringify(value.slice(-12)),{keychainAccessible:SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY})}
