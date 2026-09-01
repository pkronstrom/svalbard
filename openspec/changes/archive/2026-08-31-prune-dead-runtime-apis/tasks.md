reviewed: true

## 1. Reconfirm dead surfaces

- [x] 1.1 Trace search-DB legacy methods and migrate assertions to current APIs
- [x] 1.2 Trace MCP introspection, descriptions, and Kiwix wrapper callers
- [x] 1.3 Trace menu filter/allocation, inspect filter, and dashboard separator callers

## 2. Delete production cruft

- [x] 2.1 Remove superseded search-DB methods and redundant tests
- [x] 2.2 Remove MCP test-only APIs, descriptions, and Kiwix wrapper
- [x] 2.3 Replace visible-entry allocation, no-op filter, variadic inspect filter, and separator map

## 3. Shrink tests

- [x] 3.1 Replace hand-rolled substring search with `strings.Contains`
- [x] 3.2 Replace duplicate ANSI stripping helpers with `tui.StripAnsi`

## 4. Verify modules

- [x] 4.1 Run canonical repository verification
