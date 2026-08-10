declare module 'react-native-zeroconf' {
  export type ZeroconfService = { name:string;host?:string;port:number;addresses?:string[];txt?:Record<string,string> };
  export default class Zeroconf {
    on(event:'resolved', listener:(service:ZeroconfService)=>void):this;
    on(event:'remove', listener:(name:string)=>void):this;
    on(event:'error', listener:(error:Error)=>void):this;
    removeAllListeners():this;
    removeDeviceListeners():void;
    scan(type?:string, protocol?:string, domain?:string):void;
    stop():void;
  }
}
