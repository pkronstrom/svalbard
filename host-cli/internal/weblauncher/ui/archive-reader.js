const readingTags=new Set("a p div span section h1 h2 h3 h4 h5 h6 ol ul li dl dt dd blockquote em i b strong small sup sub code pre br hr img figure figcaption table tbody thead tfoot tr td th caption details summary".split(" "));
// The isolated frame host has archive content and PDF.js, with no dashboard API.
function archiveFrameURL(url) {
  if(!url.pathname.startsWith("/content/"))return null;
  const frameURL=new URL(location.origin);
  frameURL.hostname="localhost";
  frameURL.pathname=url.pathname;frameURL.hash=url.hash;
  return frameURL;
}
function buildReadingNode(data,item,current) {
  if(!data.tag)return document.createTextNode(data.text || "");
  if(!readingTags.has(data.tag))return document.createTextNode("");
  const el=document.createElement(data.tag), attrs=data.attrs || {};
  for(const key of ["alt","title","lang","dir","colspan","rowspan"])if(attrs[key]) el.setAttribute(key,attrs[key]);
  if(attrs.layout)el.classList.add(...attrs.layout.split(" ").map(name=>"archive-"+name));
  if(attrs.width)el.setAttribute("width",attrs.width);
  if(attrs.height)el.setAttribute("height",attrs.height);
  if(attrs.align)el.setAttribute("align",attrs.align);
  if(attrs.valign)el.setAttribute("valign",attrs.valign);
  if(attrs.id)el.id="svb-article-"+attrs.id;
  if(el.tagName==="IMG" && attrs.path) {
    el.src="/api/archives/"+item.service+"/image?"+new URLSearchParams({path:attrs.path});el.loading="lazy";el.decoding="async";
    el.onerror=()=>{const alt=node("span","quiet",el.alt || "Image unavailable in reading view");el.replaceWith(alt);};
  }
  if(el.tagName==="A") {
    if(Object.hasOwn(attrs,"path")){
      const params=new URLSearchParams({source:item.source,path:attrs.path,label:data.children?.find(n=>n.text)?.text || item.label});
      if(attrs.anchor)params.set("anchor",attrs.anchor);
      el.href="#"+item.route+"?"+params;
      if(attrs.anchor && attrs.path===current)el.onclick=event=>{
        const target=byId("svb-article-"+attrs.anchor);
        if(target){event.preventDefault();target.scrollIntoView();}
      };
    }else if(attrs.external && /^https?:\/\//i.test(attrs.external)){
      el.href=attrs.external;el.target="_blank";el.rel="noopener noreferrer";
    }
  }
  for(const child of data.children || [])el.append(buildReadingNode(child,item,current));
  return el;
}
async function adaptArchive(item, frame, url) {
  const tools=byId("archive-tools"), results=byId("archive-results");
  const article=node("article","archive-text");
  article.hidden=true;results.after(article);article.id="archive-text";
  const modes=node("label","archive-style");
  const toggle=node("input");toggle.type="checkbox";toggle.setAttribute("role","switch");
  toggle.disabled=true;
  modes.append(toggle,document.createTextNode("Original layout"));tools.prepend(modes);
  const preference="svalbard-original:"+inventory.drive+":"+item.source;
  let preferOriginal=false;
  try {preferOriginal=sessionStorage.getItem(preference)==="1";} catch {}
  toggle.onchange=()=>{
    try {sessionStorage.setItem(preference,toggle.checked?"1":"0");} catch {}
    switchView(toggle.checked);
  };
  const originalAddress=url.pathname==="/viewer" && url.hash
    ? new URL("/content/"+encodeURIComponent(decodeURIComponent(url.hash.slice(1)))+"/",url).href : url.href;
  const current=(new URLSearchParams(location.hash.split("?")[1]).get("path") || "").replace(/^\/+/,"");
  let available=false, fileCatalogue=false;
  function switchView(native) {
    if(!native && !available)return;
    toggle.checked=native;
    article.hidden=native;frame.hidden=!native;
    if(native && !frame.hasAttribute("src")) {
      // Mount at the definitive URL. Assigning src to an already mounted blank
      // frame creates an extra Back step before the catalogue.
      const loaded=frame.cloneNode(false);
      loaded.src=(fileCatalogue && archiveFrameURL(new URL(originalAddress))) || originalAddress;
      frame.replaceWith(loaded);
      frame=loaded;
    }

  }
  try {
    const data=await request("/api/archives/"+item.service+"/article?"+new URLSearchParams({path:current}));
    if(!tools.isConnected)return;
    fileCatalogue=Boolean(data.fileCatalogue);
    if(!data.nodes?.length) { modes.title="This archive provides its original layout";switchView(true);return; }
    available=true;toggle.disabled=false;
    if(data.lang)article.lang=data.lang;
    if(["rtl","ltr","auto"].includes(data.dir))article.dir=data.dir;
    const body=node("div","archive-body");
    if(data.title)body.append(node("h1","archive-title",data.title));
    for(const child of data.nodes)body.append(buildReadingNode(child,item,current));
    article.append(body);
    const headings=body.querySelectorAll("h2[id],h3[id]");
    if(headings.length>1){
      // A contents margin beside the article on wide screens, a fold above it otherwise.
      const contents=node("details","article-contents");contents.append(node("summary","","Contents"));
      contents.open=matchMedia("(min-width:1200px)").matches;
      const list=node("ol");
      for(const heading of headings){
        const row=node("li",heading.tagName==="H3"?"sub":""),go=node("button","text-button",heading.textContent);
        go.onclick=()=>{heading.scrollIntoView({block:"start"});if(!matchMedia("(min-width:1200px)").matches)contents.open=false;};
        row.append(go);list.append(row);
      }
      contents.append(list);article.prepend(contents);
    }
    switchView(preferOriginal);
    const anchor=new URLSearchParams(location.hash.split("?")[1]).get("anchor");
    if(anchor)byId("svb-article-"+anchor)?.scrollIntoView();
  } catch {
    if(tools.isConnected){toggle.disabled=true;switchView(true);}
  }
}
