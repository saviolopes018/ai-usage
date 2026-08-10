import { AppState, NativeModules } from 'react-native';
import { useCallback, useEffect, useRef, useState } from 'react';
import Zeroconf, { ZeroconfService } from 'react-native-zeroconf';
import { DiscoveredDevice } from './types';

export type DiscoveryState = 'unsupported'|'scanning'|'ready'|'error';
export type ServiceRecord = { serviceName:string; device:DiscoveredDevice; seenAt:number };

const SERVICE_TTL = 90_000;
const SWEEP_INTERVAL = 15_000;
const RESCAN_INTERVAL = 30_000;

export function useDeviceDiscovery() {
  const supported = Boolean(NativeModules.RNZeroconf);
  const [devices,setDevices]=useState<DiscoveredDevice[]>([]);
  const [state,setState]=useState<DiscoveryState>(supported?'scanning':'unsupported');
  const restartRef=useRef<()=>void>(()=>undefined);

  useEffect(()=>{
    if (!supported) return;
    const zeroconf=new Zeroconf();
    const records=new Map<string,ServiceRecord>();
    let active=AppState.currentState==='active';
    let restartTimer:ReturnType<typeof setTimeout>|null=null;

    const publish=()=>{
      const visible=reconcileServices([...records.values()],Date.now(),SERVICE_TTL);
      setDevices(visible);
      if(active)setState(visible.length?'ready':'scanning');
    };
    const clear=()=>{records.clear();setDevices([])};
    const scan=()=>{
      if(!active)return;
      if(restartTimer)clearTimeout(restartTimer);
      zeroconf.stop();
      setState('scanning');
      restartTimer=setTimeout(()=>zeroconf.scan('ai-usage','tcp','local.'),250);
    };
    const restart=()=>{clear();scan()};
    restartRef.current=restart;

    const resolved=(service:ZeroconfService)=>{
      const id=service.txt?.id;
      if(!id||!service.port||service.txt?.version!=='1')return;
      const address=pickAddress(service.addresses??[],service.host);
      if(!address)return;
      records.set(service.name,{serviceName:service.name,device:{id,name:service.name,endpoint:`${formatHost(address)}:${service.port}`},seenAt:Date.now()});
      publish();
    };
    const removed=(name:string)=>{records.delete(name);publish()};
    zeroconf.on('resolved',resolved);
    zeroconf.on('remove',removed);
    zeroconf.on('error',()=>setState('error'));

    scan();
    const sweep=setInterval(()=>{
      const cutoff=Date.now()-SERVICE_TTL;
      let changed=false;
      for(const [name,record] of records)if(record.seenAt<cutoff){records.delete(name);changed=true}
      if(changed)publish();
    },SWEEP_INTERVAL);
    // A fresh query detects Wi-Fi/IP changes even when iOS omits remove events.
    const periodicRescan=setInterval(()=>{if(active)scan()},RESCAN_INTERVAL);
    const appSubscription=AppState.addEventListener('change',next=>{
      active=next==='active';
      if(active)restart();else{if(restartTimer)clearTimeout(restartTimer);zeroconf.stop()}
    });
    return()=>{
      appSubscription.remove();clearInterval(sweep);clearInterval(periodicRescan);
      if(restartTimer)clearTimeout(restartTimer);
      zeroconf.stop();zeroconf.removeAllListeners();zeroconf.removeDeviceListeners();records.clear();
    };
  },[supported]);

  const restart=useCallback(()=>restartRef.current(),[]);
  return {devices,state,restart};
}

export function reconcileServices(records:ServiceRecord[],now:number,ttl:number):DiscoveredDevice[]{
  const byID=new Map<string,ServiceRecord>();
  for(const record of records){
    if(now-record.seenAt>ttl)continue;
    const current=byID.get(record.device.id);
    if(!current||record.seenAt>current.seenAt||(record.seenAt===current.seenAt&&record.serviceName.localeCompare(current.serviceName)<0))byID.set(record.device.id,record);
  }
  return [...byID.values()].map(record=>record.device).sort((a,b)=>a.name.localeCompare(b.name));
}

function pickAddress(addresses:string[],host?:string):string|null {
  return addresses.find(isPrivateIPv4)??addresses.find(isIPv4)??host?.replace(/\.$/,'')??addresses.find(address=>address.includes(':'))??null;
}
function isIPv4(host:string):boolean {const p=host.split('.').map(Number);return p.length===4&&p.every(part=>Number.isInteger(part)&&part>=0&&part<=255)}
function isPrivateIPv4(host:string):boolean {const p=host.split('.').map(Number);return isIPv4(host)&&(p[0]===10||(p[0]===172&&p[1]>=16&&p[1]<=31)||(p[0]===192&&p[1]===168))}
function formatHost(host:string):string{return host.includes(':')?`[${host}]`:host}
