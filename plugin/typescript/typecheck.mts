import { Registry, schemaCodec, call, Session, healthMethod, type Method } from './index.mjs';
const codec = schemaCodec<{value:string}>({type:'object',properties:{value:{type:'string'}},required:['value']});
const method: Method<{value:string},{value:string}> = {contract:{name:'example.echo',version:'v1'},operation:{name:'echo'},input:codec,output:codec};
const registry = new Registry({id:'example/types',revision:'r1'}).register(method, async input => input);
registry.guest({authorize(){}});
declare const session: Session;
const output: {value:string} = await call(session,method,{value:'ok'});
await call(session,healthMethod,{});
// @ts-expect-error wrong payload type
await call(session,method,{value:123});
void output;
