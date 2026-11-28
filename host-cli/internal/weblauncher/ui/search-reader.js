/* Vault search results and the articles they open. */
function searchResult(item, position) {
  const row = node("li");
  const title = node("a", "result-link", item.title || item.label);
  const owner = [...entries.values()].find(entry=>entry.source===item.source || (entry.source?.endsWith(".zim") && item.source.startsWith(entry.source+"/")));
  const params = new URLSearchParams({source:item.source,path:item.path,label:item.title || item.label});
  title.href = "#" + (owner?.route || "read") + "?" + params;
  title.onclick = event => {
    if(event.button || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey)return;
    event.preventDefault();
    history.replaceState({query:shownQuery,position,scroll:scrollY},"","#search?"+new URLSearchParams({q:shownQuery}));
    history.pushState({query:shownQuery,position,fromSearch:true},"",title.hash);
    window.scrollTo(0,0);renderRoute();
  };
  row.append(title, node("p", "quiet", item.label));
  if (item.snippet) row.append(node("p", "", item.snippet));
  return row;
}

async function openArticle(params,revision) {
  const label=params.get("label") || params.get("source");
  byId("reader").hidden=false;
  byId("reader-bar").hidden=false;
  byId("reader-title").textContent=label;
  byId("reader-title").focus({preventScroll:true});
  byId("reader-link").hidden=true;
  byId("back-to-search").textContent=(history.state?.fromSearch || history.state?.fromArchiveSearch) ? "← Results" : "← " + (selection ? displayTitle(selection) : "Catalogue");
  byId("search-disclosure").replaceChildren();
  note(byId("reader-status"),"Opening original…");
  try {
    const source=params.get("source");
    const link=await request("/api/read",{source,path:params.get("path")});
    if(revision!==searchRead)return;
    if(!showReader(label,link.url,source.startsWith("books/")?"documents":"browse",true))throw Error("The result has no local reader address.");
    articleService=link.service;
    if (link.service && unavailableReader(states.get(link.service))) states.delete(link.service);
    byId("reader-link").hidden=false;
    if (link.service) {
      const stop = button("Stop serving", "quiet-button", event => stopSearchReader(event.currentTarget));
      byId("search-disclosure").append(focusControl());
      serviceDisclosure(byId("search-disclosure"), "search-inspector-panel", {status:"running"}, stop);
      schedulePoll(250);
    }
    note(byId("reader-status"),"");
  } catch(error) { if(revision===searchRead)note(byId("reader-status"),error.message,true); }
}
function leaveArticle() {
  if(history.state?.fromSearch || history.state?.fromArchiveSearch) history.back();
  else location.hash=selection?.route || "overview";
}

function unavailableReader(state) {
  return ["stopping", "stopped", "failed"].includes(state?.status);
}

async function stopSearchReader(stop) {
  const service = articleService;
  if (!service) return;
  stop.disabled = true;
  try {
    const state = await request("/api/services/" + service + "/stop", {});
    showState(service, state);
  } catch (error) {
    if (articleService === service) note(byId("reader-status"), error.message, true);
  } finally {
    stop.disabled = false;
  }
}

async function runSearch() {
  ++searchRead;
  const query = byId("query").value.trim();
  const submitted = ++searchRequest;
  shownQuery = query;
  if (route === "search" && history.state?.query !== query) {
    history.replaceState({query}, "", "#search?" + new URLSearchParams({q:query}));
  }
  const button = byId("search-form").querySelector("button");
  const results = byId("search-results");
  const status = byId("search-status");
  button.disabled = true;
  results.hidden = true;
  note(status, "Searching…");
  try {
    const data = await request("/api/search", {query});
    if (submitted !== searchRequest) return;
    results.replaceChildren(...data.hits.map(searchResult));
    results.hidden = !data.hits.length;
    const count = data.hits.length;
    const summary = count ? "Showing " + count + (count === 1 ? " result" : " results") : "No matching articles or documents.";
    note(status, data.status || summary);
  } catch (error) {
    if (submitted === searchRequest) {
      shownQuery = "";
      note(status, error.message, true);
    }
  } finally {
    if (submitted === searchRequest) button.disabled = false;
  }
}
