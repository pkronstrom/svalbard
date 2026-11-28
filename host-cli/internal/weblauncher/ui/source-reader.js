/* Library, skill and Markdown sources: tabs, file picker and highlighted text. */
// Colour a bounded opening excerpt; long minified bundles stay cheap to display.
function highlightSource(body, text) {
  const first = text.slice(0, 16000);
  const pattern = /("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')|(\/\/[^\n]*|\/\*[\s\S]*?\*\/)|\b(?:async|await|break|case|class|const|default|else|export|false|for|function|if|import|let|new|null|return|throw|true|typeof|var|while)\b/g;
  const fragments = document.createDocumentFragment();
  let offset = 0;
  for (const match of first.matchAll(pattern)) {
    fragments.append(first.slice(offset, match.index), node("span", match[1] ? "literal" : match[2] ? "comment" : "keyword", match[0]));
    offset = match.index + match[0].length;
  }
  fragments.append(text.slice(offset));
  body.replaceChildren(fragments);
}

function sourceSection(part) {
  if (part.label.startsWith("Examples/")) return "Examples";
  if (part.label.startsWith("references/")) return "References";
  if (part.label.startsWith("scripts/") || part.label === "Source") return "Files";
  return part.label;
}
async function renderSource(item, section, selectedPart) {
  const panel = byId("source-view");
  const sections = [...new Set([...(item.icon === "library" ? ["Overview"] : []), ...item.parts.map(sourceSection)])];
  section ??= sections[0];
  const parts = item.parts.filter(part=>sourceSection(part)===section);
  const active = selectedPart || parts[0];
  const tabs = node("nav", "source-tabs");
  tabs.setAttribute("aria-label", "Resource views");
  for (const name of sections) {
    const tab = button(name, "secondary", () => renderSource(item,name));
    tab.setAttribute("aria-pressed", String(name === section));
    tabs.append(tab);
  }
  panel.hidden=false;
  panel.replaceChildren(tabs);
  if (!active) {
    const overview=node("article","prose");
    overview.append(node("h2","",item.label),node("p","",item.description || item.note),node("p","quiet","No overview document is installed with this package."));
    panel.append(overview);
    return;
  }
  if (item.icon==="library" && section==="Files") {
    const tree=node("details","source-tree");tree.append(node("summary","","Package files"));
    for(const part of parts)tree.append(button(part.path,"quiet-button",()=>renderSource(item,section,part)));
    panel.append(tree);
  } else if(parts.length>1) {
    const picker=node("select");picker.setAttribute("aria-label","File");
    parts.forEach((part,i)=>picker.add(new Option(part.path,String(i))));
    picker.selectedIndex=parts.indexOf(active);
    picker.onchange=()=>renderSource(item,section,parts[picker.selectedIndex]);
    panel.append(picker);
  }
  const file = node("p", "item-recipe", active.path);
  const isMarkdown=/\.md$/i.test(active.path);
  const body = node(isMarkdown ? "article" : "pre", isMarkdown ? "prose" : "source-text", "Loading…");
  panel.append(file,body);
  try {
    const data = await request("/api/source?" + new URLSearchParams({id:item.id, view:active.label}));
    if (!body.isConnected) return;
    body.tabIndex = 0;
    if (/\.(m?js|json|cjs)$/.test(active.path)) highlightSource(body, data.text);
    else if (isMarkdown) {
      const {prose}=await import("/views.js");
      const fragment=prose(data.text);
      const base=new URL("/"+active.path,"https://source.invalid");
      for(const link of fragment.querySelectorAll("a[href]")) {
        const address=new URL(link.getAttribute("href"),base);
        if(address.origin!==base.origin)continue;
        const path=decodeURIComponent(address.pathname.slice(1));
        const part=item.parts.find(part=>part.path===path);
        link.removeAttribute("target");
        if(part){
          link.href="#"+item.route;
          link.onclick=event=>{event.preventDefault();if(part===active && address.hash) body.querySelector("#"+CSS.escape(decodeURIComponent(address.hash.slice(1))))?.scrollIntoView();else renderSource(item,sourceSection(part),part);};
        }else link.removeAttribute("href");
      }
      body.replaceChildren(fragment);
      if(item.format==="MD"||item.icon==="skill"){
        const headings=[...body.querySelectorAll("h2,h3")];
        if(headings.length>1) {
          const contents=node("details","article-contents");contents.append(node("summary","","Contents"));
          for(const heading of headings)contents.append(button(heading.textContent,"quiet-button",()=>heading.scrollIntoView()));
          body.prepend(contents);
        }
      }
      const raw=button("Original text","quiet-button",()=>{body.replaceChildren(node("pre","source-text",data.text));raw.remove();});
      tabs.append(raw);
      const example=[...body.querySelectorAll("h2,h3")].find(heading=>/^examples?$/i.test(heading.textContent.trim()));
      if(example && !sections.includes("Examples")) tabs.append(button("Examples","secondary",()=>{example.tabIndex=-1;example.focus();}));
    } else body.textContent = data.text;
    if (section==="Examples" && /\.html?$/i.test(active.path) && !data.truncated) {
      const preview=button("Preview markup","secondary",()=>{
        const frame=node("iframe","example-frame");
        frame.setAttribute("sandbox","");frame.title=active.path;
        frame.srcdoc='<meta http-equiv="Content-Security-Policy" content="default-src &apos;none&apos;; style-src &apos;unsafe-inline&apos;">'+data.text;
        body.after(frame);preview.remove();
      });
      tabs.append(preview);
    }
    if (data.truncated) file.append(" · First 256 KiB");
    const copy = button("Copy text", "quiet-button", async () => {
      try { await navigator.clipboard.writeText(data.text); copy.textContent = "Copied"; }
      catch { body.focus(); }
    });
    tabs.append(copy);
  } catch (error) {note(body, error.message, true);}
}
