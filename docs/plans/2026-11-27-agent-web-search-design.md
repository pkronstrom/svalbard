# Keyless web search for Svalbard agents

Draft design · 2026-11-27

## User path

In the drive's existing Settings screen, turn on **Web search**. The setting is off on a fresh vault. It applies to the next client launch, which tells the user whether search queries may go to DuckDuckGo or to a configured SearXNG server. A question about current information can then return cited web URLs. The same client can still look up a manual or wiki page on the drive.

```text
Terminal clients ── stdio ───────┐
Open WebUI ───── Streamable HTTP ┴─ Svalbard tool server
                                      ├─ Vault search → local holdings
                                      └─ Web search → DuckDuckGo or SearXNG
```

Use the Local AI settings group for the toggle and its CLI equivalent. Leave **SearXNG URL** empty to select DuckDuckGo. Missing or unreadable settings resolve to off. The helper sends no request during tool discovery, startup, or vault lookup. An enabled setting allows calls in that launched session. The terminal instructions and MCP descriptions direct models to prefer the vault and reserve network queries for an explicit web request or a request for current public information. The runtime enforces this gate for Svalbard's search helper. General-purpose shell commands still have the host's network permissions.

## Search provider and tool boundary

Use Go's HTTP client and the HTML parser already shipped in the drive runtime. DuckDuckGo provides a public HTML endpoint. Both GET and POST returned a bot challenge during implementation testing, so support a user-chosen SearXNG endpoint with JSON enabled as well. Each call goes to the single selected provider. Use HTTPS or an HTTP loopback instance, and refuse redirects. Neither provider needs a key in this configuration. [DuckDuckGo HTML search](https://duckduckgo.com/duckduckgo-help-pages/features/non-javascript) · [SearXNG JSON API](https://docs.searxng.org/dev/search_api.html)

A small Go client owns deadlines, errors and output limits. Expose it through `./run web-search <query>` and the `web_search` capability in the existing MCP server. Keep one page of results, three hits by default and at most five. Return title, public URL and short plaintext snippet with a bounded total response. Search sends only the query and ordinary engine parameters. Query text can still be present in the client's existing conversation history.

The launcher explicitly enables search in terminal MCP registration and oh-my-pi's allowlist. The MCP process checks the setting again when it starts. The direct CLI consults the same setting. Mark the MCP action as open-world so clients can distinguish its network use. On Open WebUI's existing loopback MCP listener, present the capability as a separately named **Web search** connection with its provider shown. Users can toggle it in a chat independently of their vault tools. Leave Open WebUI's native search disabled and the standalone HTTP/SSE endpoints vault-only. [Svalbard MCP factory](../../drive-runtime/internal/mcp/server.go) · [Standalone HTTP/SSE entry points](../../drive-runtime/cmd/svalbard-drive/main.go) · [Open WebUI tool connections](../../drive-runtime/internal/openwebui/openwebui.go)

Report a timeout, an unrecognized response or a provider refusal as search unavailable. Keep an explicit empty response distinct from a broken response. DuckDuckGo may change its HTML or block automated traffic. SearXNG instances can disable JSON or report errors from their configured search engines. [SearXNG JSON API](https://docs.searxng.org/dev/search_api.html)

## Reading pages

Ship search with snippets and links first. Add a separate `web_fetch` only once there is a demonstrated need to read an entire source for citation. That tool would take one public HTTP(S) URL, return bounded plain text, and check the destination on every connection and redirect. It would use a request with no browser cookies, credentials or local network access. Search results and fetched text are source material the model can quote, never instructions for changing settings or accessing the drive.

## Acceptance

- On a minimal vault and with the switch off, a request for current information generates no Svalbard web search traffic. Tool listings omit the capability and both the direct CLI and a stale MCP call are refused. Repeat after switching it off and restarting, and with old and invalid settings. Standalone HTTP and SSE omit the tool even when the setting is on.
- With the switch on, make a current-information query from the direct CLI, pi and Open WebUI. Answers cite actual returned URLs. Confirm the browser shows which provider is enabled and can toggle it for a chat. Check tool discovery in Goose, OpenCode and oh-my-pi, including its allowlist.
- Run with a supported small tool-calling model at 8K. Check that an offline vault lookup still reaches the correct tool and has room to answer. Check that unplugging the network produces a bounded failure and leaves the same chat usable.
- Exercise capped output, malformed replies, rate limits and cancellation with stand-in providers. Check the requested destination and verify the adapter does not fall through to another provider. Record live provider results separately from fixture tests.
