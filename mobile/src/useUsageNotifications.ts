import { useEffect, useRef, useState } from 'react';
import { ConnectionState, NotificationPreferences, ProviderUsage, UsageSnapshot } from './types';
import { DEFAULT_NOTIFICATION_PREFERENCES, loadNotificationPreferences, loadUsageHistory, saveNotificationPreferences, saveUsageHistory } from './storage';

export type UsageAlert={key:string;title:string;body:string};
type NotificationsModule=typeof import('expo-notifications');

export function useUsageNotifications(snapshot:UsageSnapshot|null,state:ConnectionState){
  const [preferences,setPreferences]=useState(DEFAULT_NOTIFICATION_PREFERENCES);
  const [loaded,setLoaded]=useState(false);
  const [historyLoaded,setHistoryLoaded]=useState(false);
  const [permission,setPermission]=useState<'unknown'|'granted'|'denied'|'unsupported'>('unknown');
  const previous=useRef<UsageSnapshot|null>(null);const emitted=useRef(new Set<string>());const scheduled=useRef<string[]>([]);
  const history=useRef<UsageSnapshot[]>([]);
  // Expo modules are registered through ExpoModulesCore and are not guaranteed
  // to appear in React Native's NativeModules object.
  const supported=Boolean(getNotifications());
  useEffect(()=>{if(!supported)return;const module=getNotifications();module?.setNotificationHandler({handleNotification:async()=>({shouldShowBanner:true,shouldShowList:true,shouldPlaySound:true,shouldSetBadge:false})})},[supported]);
  useEffect(()=>{loadNotificationPreferences().then(setPreferences).finally(()=>setLoaded(true))},[]);
  useEffect(()=>{loadUsageHistory().then(value=>{history.current=value}).finally(()=>setHistoryLoaded(true))},[]);
  useEffect(()=>{if(!historyLoaded||!snapshot)return;const known=history.current.some(item=>item.updatedAt===snapshot.updatedAt);if(!known){history.current=[...history.current,snapshot].slice(-12);void saveUsageHistory(history.current)}},[historyLoaded,snapshot]);
  useEffect(()=>{if(!loaded||!historyLoaded||!preferences.enabled||!snapshot||!supported){previous.current=snapshot;return}
    const module=getNotifications();if(!module)return;
    const alerts=computeUsageAlerts(previous.current,snapshot,preferences,Date.now(),emitted.current,history.current);
    for(const alert of alerts)void module.scheduleNotificationAsync({content:{title:alert.title,body:alert.body,sound:'default'},trigger:null});
    previous.current=snapshot;
  },[loaded,historyLoaded,preferences,snapshot,state,supported]);
  useEffect(()=>{if(!loaded||!preferences.enabled||!snapshot||!supported)return;const module=getNotifications();if(!module)return;
    void (async()=>{for(const id of scheduled.current)await module.cancelScheduledNotificationAsync(id).catch(()=>undefined);scheduled.current=[];if(!preferences.windowReset)return;
      for(const provider of snapshot.providers)for(const [name,label,window] of [['5h','5 horas',provider.fiveHour],['weekly','semanal',provider.weekly]] as const){if(name==='5h'&&!preferences.fiveHourAlerts||name==='weekly'&&!preferences.weeklyAlerts)continue;let date=window?.resetsAt?new Date(window.resetsAt):null;if(!date||date.getTime()<=Date.now()+60_000)continue;if(preferences.quietHours)date=afterQuietHours(date,preferences.quietStartHour,preferences.quietEndHour);const id=await module.scheduleNotificationAsync({content:{title:`${display(provider)} renovado`,body:`A janela ${label} foi renovada.`},trigger:{type:module.SchedulableTriggerInputTypes.DATE,date}});scheduled.current.push(id)}})();
  },[loaded,preferences.enabled,preferences.windowReset,preferences.fiveHourAlerts,preferences.weeklyAlerts,preferences.quietHours,preferences.quietStartHour,preferences.quietEndHour,snapshot,supported]);
  const update=async(next:NotificationPreferences)=>{if(next.enabled&&supported){const module=getNotifications();const result=await module?.requestPermissionsAsync();setPermission(result?.granted?'granted':'denied');if(!result?.granted)next={...next,enabled:false}}else if(next.enabled&&!supported){setPermission('unsupported');next={...next,enabled:false}}await saveNotificationPreferences(next);setPreferences(next)};
  const test=async()=>{if(!supported){setPermission('unsupported');return false}const module=getNotifications();if(!module)return false;const result=await module.requestPermissionsAsync();setPermission(result.granted?'granted':'denied');if(!result.granted)return false;await module.scheduleNotificationAsync({content:{title:'Codex atingiu 75%',body:'Teste simulado · janela semanal com 25% restante.',sound:'default'},trigger:null});return true};
  return {preferences,update,test,permission,supported};
}

export function computeUsageAlerts(previous:UsageSnapshot|null,current:UsageSnapshot,prefs:NotificationPreferences,now:number,emitted:Set<string>,history:UsageSnapshot[]=[]):UsageAlert[]{
  if(!previous||prefs.quietHours&&isQuietHour(new Date(now).getHours(),prefs.quietStartHour,prefs.quietEndHour))return[];const alerts:UsageAlert[]=[];
  for(const provider of current.providers){const before=previous.providers.find(item=>item.provider===provider.provider);
    if(prefs.providerUnavailable&&before?.available&&!provider.available)add(alerts,emitted,`${provider.provider}:unavailable`,`${display(provider)} indisponível`,'O agent não conseguiu obter uma leitura recente.');
    if(provider.available){emitted.delete(`${provider.provider}:unavailable`);const stale=now-new Date(provider.observedAt).getTime()>15*60_000;const staleKey=`${provider.provider}:stale`;if(prefs.staleData&&stale)add(alerts,emitted,staleKey,`Dados antigos do ${display(provider)}`,'A última leitura real tem mais de 15 minutos.');else if(!stale)emitted.delete(staleKey);
      for(const [windowName,window] of [['5h',provider.fiveHour],['weekly',provider.weekly]] as const){if(windowName==='5h'&&!prefs.fiveHourAlerts||windowName==='weekly'&&!prefs.weeklyAlerts)continue;const oldWindow=windowName==='5h'?before?.fiveHour:before?.weekly;if(!window||!oldWindow)continue;for(const threshold of prefs.thresholds){const key=`${provider.provider}:${windowName}:${threshold}:${window.resetsAt}`;if(oldWindow.usedPercentage<threshold&&window.usedPercentage>=threshold)add(alerts,emitted,key,`${display(provider)} atingiu ${threshold}%`,`${windowName==='5h'?'Janela de 5 horas':'Janela semanal'} com ${Math.round(window.remainingPercentage)}% restante.`)}if(prefs.predictiveAlerts){const hoursToEmpty=estimateHoursToEmpty(history,current,provider.provider,windowName);const resetHours=(new Date(window.resetsAt).getTime()-now)/3_600_000;if(hoursToEmpty<=12&&hoursToEmpty<resetHours)add(alerts,emitted,`${provider.provider}:${windowName}:prediction:${window.resetsAt}`,`${display(provider)} pode esgotar em breve`,`${windowName==='5h'?'Janela de 5 horas':'Janela semanal'}: projeção baseada nas últimas leituras indica esgotamento em cerca de ${Math.max(1,Math.round(hoursToEmpty))} h.`)}}
    }}return alerts;
}
function add(alerts:UsageAlert[],emitted:Set<string>,key:string,title:string,body:string){if(emitted.has(key))return;emitted.add(key);alerts.push({key,title,body})}
function display(provider:ProviderUsage){return provider.provider==='codex'?'Codex':provider.provider==='claude'?'Claude':provider.provider}
export function isQuietHour(hour:number,start:number,end:number){return start===end||start<end?hour>=start&&hour<end:hour>=start||hour<end}
export function estimateHoursToEmpty(history:UsageSnapshot[],current:UsageSnapshot,providerName:string,windowName:'5h'|'weekly'){
  const samples=[...history.filter(item=>item.updatedAt!==current.updatedAt),current].slice(-12).map(item=>{const provider=item.providers.find(value=>value.provider===providerName);const window=windowName==='5h'?provider?.fiveHour:provider?.weekly;return provider&&window?{at:new Date(provider.observedAt).getTime(),used:window.usedPercentage,remaining:window.remainingPercentage,reset:window.resetsAt}:null}).filter((item):item is NonNullable<typeof item>=>Boolean(item)).filter(item=>item.reset===(windowName==='5h'?current.providers.find(value=>value.provider===providerName)?.fiveHour?.resetsAt:current.providers.find(value=>value.provider===providerName)?.weekly?.resetsAt)).sort((a,b)=>a.at-b.at);
  const rates:number[]=[];for(let i=1;i<samples.length;i++){const hours=(samples[i].at-samples[i-1].at)/3_600_000;const delta=samples[i].used-samples[i-1].used;if(hours>0&&delta>0&&delta<=50)rates.push(delta/hours)}if(rates.length<2)return Infinity;rates.sort((a,b)=>a-b);const rate=rates[Math.floor(rates.length/2)];return samples[samples.length-1].remaining/rate
}
function afterQuietHours(date:Date,start:number,end:number){if(!isQuietHour(date.getHours(),start,end))return date;const shifted=new Date(date);shifted.setHours(end,0,0,0);if(start>end&&date.getHours()>=start)shifted.setDate(shifted.getDate()+1);return shifted}
function getNotifications():NotificationsModule|null{try{
  // Delayed loading keeps older development builds usable until the native module is rebuilt.
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  return require('expo-notifications') as NotificationsModule
}catch{return null}}
