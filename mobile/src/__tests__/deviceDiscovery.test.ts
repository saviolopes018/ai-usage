import { reconcileServices, ServiceRecord } from '../useDeviceDiscovery';

const device = (serviceName:string,id:string,endpoint:string,seenAt:number):ServiceRecord => ({serviceName,seenAt,device:{id,name:serviceName,endpoint}});

describe('mDNS reconciliation',()=>{
  test('deduplicates multiple service names by stable device id and keeps the newest endpoint',()=>{
    const devices=reconcileServices([
      device('Mac.local','same-id','192.168.1.2:9876',100),
      device('Mac (2).local','same-id','192.168.1.9:9876',200),
    ],250,1_000);
    expect(devices).toEqual([{id:'same-id',name:'Mac (2).local',endpoint:'192.168.1.9:9876'}]);
  });

  test('drops services that disappeared without a remove event',()=>{
    expect(reconcileServices([device('Old Mac','old-id','192.168.1.2:9876',100)],1_201,1_000)).toEqual([]);
  });

  test('keeps a duplicate alive when only one announcement is stale',()=>{
    const devices=reconcileServices([
      device('Old alias','same-id','192.168.1.2:9876',100),
      device('Current alias','same-id','192.168.1.3:9876',1_100),
    ],1_200,1_000);
    expect(devices[0].endpoint).toBe('192.168.1.3:9876');
  });
});
