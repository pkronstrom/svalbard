---
name: vault-research
description: Answer a question from the drive's own archives and documents (offline Wikipedia, manuals, medical and survival references, PDFs) with the search tools, cite what was read, and say plainly when the drive does not have it.
requires: []
---

## Steps

The tools are the drive's `vault_sources`, `search` and `search_read`. Your
tool list shows them with a prefix, such as `mcp__svalbard__search`.

1. Call `vault_sources` once to see what the drive holds. Note the sources
   that fit the question.
2. Call `search` with two to four specific words, not a sentence:
   `amoxicillin dose`, not `what dose of amoxicillin should an adult take`.
   Give `source` to search one archive when the question belongs to it.
3. Call `search_read` on the one to three results whose titles fit best.
   A long article comes in parts: call again with the offset it names when
   the part you need is not there yet.
4. Answer only from what you read. Cite each fact by its title and source,
   as `Water treatment (Outdoors Stack Exchange)`, or by page for a PDF, as
   `TM 9-8000, p. 112`.
5. If two searches with different words find nothing that answers the
   question, tell the user the drive does not have it. Anything you add from
   your own knowledge, say so: it was not checked against the drive.
6. Check your result when you saved the answer to a file. It must print
   `ok`; `<folder>` is the skill folder that `skill_read` returned (keep the quotes
   around it: a drive's path can hold spaces):

   ```sh
   sh "<folder>"/examples/check.sh answer.md
   ```

## Without the drive's tools

An agent with a shell but without these tools (one not started from the
drive's menu) calls them through the script, with the same arguments:

```sh
sh "<folder>"/scripts/mcp_call.sh vault_sources
sh "<folder>"/scripts/mcp_call.sh search '{"query": "water filter", "limit": 5}'
sh "<folder>"/scripts/mcp_call.sh search_read '{"source": "<source from a result>", "path": "<path from a result>"}'
```

## Good searches

- Name the thing, then what you want of it: `tetanus vaccine schedule`,
  `rocket stove dimensions`, `potato blight treatment`.
- Try a synonym or the technical term before giving up: `heart attack`,
  then `myocardial infarction`.
- Medical, legal or safety answers: give the source's words and its
  warnings, not a summary that drops them.
- The drive's databases (`vault_databases`, `query_sql`) hold tables such
  as nutrition values; use them for numbers when a source names one.
