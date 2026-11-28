let registry = {records:[], jobs:{}, version:"", draining:false, terminals:0};
let lastRegistry = "";
let drawerTrigger;
let serviceUnknown = true;
const serviceNames = {"maps":"Map viewer", "documents":"Document reader", "apps":"App server", "workspace":"Saved apps", "wiki":"Personal wiki", "share":"Local network share", "mcp-serve":"MCP server", "open-webui":"Open WebUI", "serve-all":"Serve everything", "inspect":"Drive inspection"};
function serviceTitle(record) { return serviceNames[record.name] || record.name; }
function jobStatus(key) {return serviceUnknown ? "unknown" : states.get(key)?.status || "stopped";}
function registerState(status) { return ({unknown:"Unknown",done:"Finished",stopped:"Stopped",starting:"Starting",running:"Running",stopping:"Stopping",failed:"Failed"})[status] || "Unknown"; }
function showRegisterRecord(record, compact) {
  const row=node("li","service-row"), info=node("div"), heading=node("strong","service-title",serviceTitle(record));
  info.append(heading,node("small","service-state",registerState(serviceUnknown?"unknown":record.status)));
  info.append(node("p","service-resources",record.resources.map(label=>label.split(/\s+[—–]\s+/)[0]).join(" · ")));
  for (const endpoint of record.endpoints || []) {
    const address=node("small","service-address",endpoint.replace(/^https?:\/\//,""));
    address.title=endpoint;info.append(address);
  }
  if (record.status==="running") info.append(node("small","service-address",runningFor(record.started)));
  row.append(info);
  if(!compact) {
    const details=node("details","service-info");
    details.dataset.record=record.id;
    details.append(node("summary","","Process and activity"));
    const deps=registry.records.filter(other=>other.id!==record.id && other.keys.some(k=>record.keys.includes(k))).map(serviceTitle);
    for(const text of ["PID · "+(record.pid || "Unavailable"), "Executable · "+(record.executable || "Unavailable"),"Dependencies · "+(deps.join(", ") || "Not reported"), "Memory / CPU · Unavailable",...(record.events || [])]) details.append(node("p","service-address",text));
    info.append(details);
  }
  const controls=node("div","service-actions");
  if(isActive(record)) {
    const stop=button("Stop","quiet-button",()=>controlServices("stop",record));
    stop.disabled=serviceUnknown||record.status==="stopping"; stop.setAttribute("aria-label","Stop "+serviceTitle(record));controls.append(stop);
  }
  if (!compact && !registry.draining && !["starting","stopping"].includes(record.status)) {
    const restart=button(record.status==="running"?"Restart":"Start","quiet-button",()=>controlServices("restart",record));
    restart.disabled=serviceUnknown;controls.append(restart);
  }
  row.append(controls);
  return row;
}
function runningFor(started) {
  const minutes=Math.max(0,Math.floor((Date.now()-Date.parse(started))/60000));
  if(!Number.isFinite(minutes) || minutes<1) return "Started just now";
  return "Running for "+(minutes<60 ? minutes+" min" : Math.floor(minutes/60)+" h "+minutes%60+" min");
}
function registryMessage(message, error=false) {
  for(const id of ["service-feedback","drawer-feedback"]) note(byId(id),message,error);
}
async function controlServices(action,record) {
  const affected=record ? registry.records.filter(other=>other.keys.some(k=>record.keys.includes(k))) : registry.records.filter(isActive);
  const names=[...new Set(affected.flatMap(r=>r.resources))].join(", ");
  const warning=affected.some(r=>r.inference) ? " Active inference or chat requests may be interrupted." : "";
  if(action!=="resume" && action!=="close" && (action==="stop-all" || action==="prepare" || warning || affected.some(r=>r.keys.length>1) || affected.length>1)) {
    const verb=action==="restart"?"Restart":action==="prepare"?"Prepare the drive and stop":"Stop";
    if(!confirm(verb+" the managed services for "+(names || "this drive")+"?"+warning+" Other tabs using these services will lose access.")) return;
  }
  try {
    const response=await request("/api/lifecycle",{action,id:record?.id || "",version:registry.version});
    registryMessage(response.message);
    if (response.canClose) {
      for (const id of ["service-feedback", "drawer-feedback"]) {
        byId(id).append(" ",button("Close dashboard","quiet-button",()=>controlServices("close")));
      }
    }
    if (action==="close") return;
  } catch(error) {registryMessage(error.message,true);}
  await poll();
}
function renderServices(force=false) {
  const running=registry.records.filter(r=>r.status==="running");
  const count=serviceUnknown?"?":String(running.length);
  byId("rail-count").hidden=count==="0";
  byId("rail-count").textContent=count;
  const label=serviceUnknown?"Service status unknown":count+" running service"+(count==="1"?"":"s");
  byId("services-nav").setAttribute("aria-label","Service activity, "+label);
  byId("service-summary").textContent=count==="0"?"Services":"Services · "+count+" running";
  byId("drawer-title").textContent="Service activity";
  const full=route==="services",drawer=byId("service-drawer").open;
  const signature=[registry.version,serviceUnknown,full,drawer,registry.draining,Math.floor(Date.now()/60000)].join("|");
  if (!force && signature===lastRegistry) return;
  lastRegistry=signature;
  for(const [listId,statusId,compact,visible] of [["services-list","services-status",false,full],["drawer-services","drawer-status",true,drawer]]) {
    if(!visible)continue;
    const list=byId(listId), opened=[...list.querySelectorAll("details[open]")].map(d=>d.dataset.record);
    const active=document.activeElement;
    const previous=list.contains(active) ? [active.closest("li")?.dataset.record,active.getAttribute("aria-label")||active.textContent] : null;
    list.replaceChildren();
    const records=registry.records.filter(r=>!compact || isActive(r));
    byId(statusId).textContent=serviceUnknown?"Waiting for the local dashboard.":registry.draining?"Preparing for removal · new launches paused":running.length?label:records.length?"No services running.":"No services have been started.";
    for(const record of records) {
      const row=showRegisterRecord(record,compact);row.dataset.record=record.id;
      if(opened.includes(record.id) && row.querySelector("details"))row.querySelector("details").open=true;
      list.append(row);
    }
    if(previous) {
      const row=[...list.children].find(el=>el.dataset.record===previous[0]);
      const control=[...(row?.querySelectorAll("button:not(:disabled),summary")||[])].find(el=>(el.getAttribute("aria-label")||el.textContent)===previous[1]);
      (control || byId(compact?"drawer-close":"services-title")).focus({preventScroll:true});
    }
  }
  for(const el of document.querySelectorAll("[data-bulk]")){
    el.disabled=serviceUnknown || (el.dataset.bulk==="stop-all" && !registry.records.some(isActive));
    if(el.dataset.bulk==="resume")el.hidden=!registry.draining;
  }
}
function initServiceDrawer() {
  byId("drive-users-check").onclick=async event=>{
    const btn=event.currentTarget;btn.disabled=true;
    try {
      const scan=await request("/api/drive-users",{});
      const names=(scan.users || []).map(p=>(p.name || "Process")+" (PID "+p.pid+")");
      note(byId("drive-users"),(names.length ? "Ownership unverified · "+names.join(", "):"No other processes reported.")+(scan.complete?"":" The system could not verify every open handle."));
    }catch(err){note(byId("drive-users"),err.message,true);}finally{btn.disabled=false;}
  };
  const dialog=byId("service-drawer");
  const open=event=>{event.preventDefault();drawerTrigger=event.currentTarget;dialog.showModal();renderServices(true);schedulePoll(0);byId("drawer-close").focus();};
  byId("services-nav").onclick=open;
  byId("service-summary").onclick=open;
  byId("drawer-close").onclick=()=>dialog.close();
  dialog.onclose=()=>drawerTrigger?.focus({preventScroll:true});
  byId("full-register").onclick=()=>dialog.close();
  for(const el of document.querySelectorAll("[data-bulk]")) el.onclick=()=>controlServices(el.dataset.bulk);
}
