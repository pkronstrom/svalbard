const readingTags=new Set("a p div span section h1 h2 h3 h4 h5 h6 ol ul li dl dt dd blockquote em i b strong small sup sub code pre br hr img figure figcaption table tbody thead tfoot tr td th caption details summary".split(" "));
function buildReadingNode(data,item,current) {
  if(!data.tag)return document.createTextNode(data.text || "");
  if(!readingTags.has(data.tag))return document.createTextNode("");
  const el=document.createElement(data.tag), attrs=data.attrs || {};
  for(const key of ["alt","title","lang","dir","colspan","rowspan"])if(attrs[key]) el.setAttribute(key,attrs[key]);
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
  const original=button("Original","quiet-button",()=>switchView(true));
  const reader=button("Reader","quiet-button",()=>switchView(false));
  const modes=node("div","archive-modes");modes.append(reader,original);tools.prepend(modes);
  const originalAddress=url.pathname==="/viewer" && url.hash
    ? new URL("/content/"+encodeURIComponent(decodeURIComponent(url.hash.slice(1)))+"/",url).href : url.href;
  const current=(new URLSearchParams(location.hash.split("?")[1]).get("path") || "").replace(/^\/+/,"");
  let available=false;
  function switchView(native) {
    if(!native && !available)return;
    original.setAttribute("aria-pressed",String(native));reader.setAttribute("aria-pressed",String(!native));
    article.hidden=native;frame.hidden=!native;
    if(native && !frame.hasAttribute("src")) {
      // Mount at the definitive URL. Assigning src to an already mounted blank
      // frame creates an extra Back step before the catalogue.
      const loaded=frame.cloneNode(false);
      loaded.src=originalAddress;
      frame.replaceWith(loaded);
      frame=loaded;
    }

  }
  try {
    const data=await request("/api/archives/"+item.service+"/article?"+new URLSearchParams({path:current}));
    if(!tools.isConnected)return;
    if(!data.nodes?.length) { reader.disabled=true;reader.title="This archive uses its original layout";switchView(true);return; }
    available=true;
    if(data.lang)article.lang=data.lang;
    if(["rtl","ltr","auto"].includes(data.dir))article.dir=data.dir;
    if(data.title)article.append(node("h2","",data.title));
    for(const child of data.nodes)article.append(buildReadingNode(child,item,current));
    const headings=article.querySelectorAll("h2[id],h3[id]");
    if(headings.length>1){
      const contents=node("details","article-contents");contents.append(node("summary","","Contents"));
      const list=node("ul");
      for(const heading of headings){const row=node("li"),go=node("button","text-button",heading.textContent);go.onclick=()=>heading.scrollIntoView();row.append(go);list.append(row);}
      contents.append(list);article.prepend(contents);
    }
    switchView(false);
    const anchor=new URLSearchParams(location.hash.split("?")[1]).get("anchor");
    if(anchor)byId("svb-article-"+anchor)?.scrollIntoView();
  } catch {
    if(tools.isConnected){reader.disabled=true;switchView(true);}
  }
}
