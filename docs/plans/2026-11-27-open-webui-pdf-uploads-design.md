# Ad hoc PDF uploads in Open WebUI

Draft design · 2026-11-27

## User path

Open Open WebUI from the drive, attach a small text-bearing PDF to a chat and ask about a specific page. The answer can cite the physical PDF page number. Text attachments still work through the same upload control. A scanned or encrypted PDF gets an actionable message. A long manual belongs in a document collection, where the host can index it and all clients can retrieve individual pages without filling the context. [PDF collections checkpoint](2026-11-26-pdf-collections-checkpoint.md)

## Attachment extraction

Use the pinned slim build's external document loader to handle uploads. Open WebUI sends raw bytes to a loopback handler owned by the launch, with a fresh local bearer token. Route PDF bytes to a bounded extraction worker with an explicit pinned `pypdf` dependency in the Open WebUI recipe. Open WebUI joins extracted document bodies into one stored text string in bypass mode, so put physical page markers in the text. For a native browser chat, use `view_file` and `grep_chat_files` to read the stored text without embeddings. Set a short context-dependent default for each read and point the browser model to those tools in its default prompt. This preserves pagination for existing text attachments. [Pinned loader dispatch](https://github.com/open-webui/open-webui/blob/v0.13.5/backend/open_webui/retrieval/loaders/main.py#L531-L549) · [Pinned attachment storage](https://github.com/open-webui/open-webui/blob/v0.13.5/backend/open_webui/routers/retrieval.py#L1722-L1745) · [Pinned attachment delivery](https://github.com/open-webui/open-webui/blob/v0.13.5/backend/open_webui/utils/tools.py) · [External loader contract](https://docs.openwebui.com/features/chat-conversations/rag/document-extraction/external/)

```text
Browser upload → Open WebUI → local extraction handler → PDF worker
                     ↑                 │
                     └──── text with physical PDF page markers
```

The pinned loader dispatch sends ordinary text to the external engine too. Reuse the pinned slim loaders so text encoding, HTML-to-text extraction and CSV formatting retain their existing defaults. Treat uploaded names as labels. Open WebUI owns the uploaded attachment and how it is retained. The extractor handles bytes for one request, uses no internet or shared document index, and leaves no working files after success, error or cancellation. The existing host PDFium path remains responsible for installed collections. OCR and Office formats are outside this slice. [Pinned slim text loaders](https://github.com/open-webui/open-webui/blob/v0.13.5/backend/open_webui/retrieval/loaders/main.py#L688-L701)

Limit upload size in Open WebUI and again at the handler, and cap page count, extracted output and worker runtime. Test memory handling independently of upload size, since a small PDF can expand into a large content stream. Use a worker with enforceable resource limits before enabling PDF extraction by default on a supported platform. The extraction request has no model or chat context. Size the per-document budget for the smallest model context in the launch and include the page markers in the count. Reject over-budget documents with a message pointing to collections. The first supported workflow is one small PDF per turn. Multiple attachments or a long prior conversation may exceed the combined budget, so verify that these cases surface a context limit instead of silently dropping pages. Use a startup fallback to existing text uploads if the worker or handler is unavailable. [Launch context floor](../../drive-runtime/internal/openwebui/openwebui.go#L251)

## Acceptance

- With outbound networking blocked, attach an ordinary two-page PDF and answer a question whose supporting text appears on the second physical page. Repeat with a mixed PDF where a scanned page precedes the relevant text page.
- Attach UTF-8 and BOM-encoded text, CSV and HTML after the change and compare the processed content with the current slim path.
- Exercise empty, scanned, encrypted, corrupt, oversized and resource-heavy PDFs. Errors leave chat working and identify the next action. Stop Open WebUI during extraction and confirm the worker and listener exit.
- Run through the pinned Open WebUI browser UI at 8K and 32K. Measure how much of the context the upload actually consumes before choosing the accepted text budget.
