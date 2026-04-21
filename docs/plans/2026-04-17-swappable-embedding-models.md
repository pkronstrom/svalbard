# Swappable Embedding Models — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make embedding models recipe-driven and swappable, with a dedicated `models/embed/` directory, model identity recorded in search.db, and a smaller default model.

**Architecture:** Embedding models get their own type (`gguf-embed`) mapped to `models/embed/`. The indexer picks the model from that directory and records its ID in search.db. Search-time validates the model is present. The `isEmbeddingModel` filename heuristic and `findEmbeddingModel` glob patterns are replaced by a simple directory scan.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), llama-server (GGUF inference)

---

### Task 1: Add `gguf-embed` type and `models/embed/` directory

**Files:**
- Modify: `host-cli/internal/toolkit/toolkit.go:52-63` (TypeDirs)
- Modify: `host-cli/internal/toolkit/toolkit.go:196` (typeDefaults — skip gguf-embed from menu)
- Modify: `host-cli/internal/toolkit/toolkit.go:202-205` (remove isEmbeddingModel)
- Modify: `host-cli/internal/catalog/embedded/recipes/models/nomic-embed-text-v1.5.yaml` (type: gguf-embed)

**Step 1: Add type mapping**

In `host-cli/internal/toolkit/toolkit.go`, add to `TypeDirs`:

```go
"gguf-embed": "models/embed",
```

**Step 2: Remove `isEmbeddingModel` function and its call**

Delete the `isEmbeddingModel` function (lines 202-205). In `writeActionsConfig`, remove the `gguf` embedding filter check (line ~288: `if e.Type == "gguf" && isEmbeddingModel(e.Filename)`). Embedding models now have type `gguf-embed` so they never enter the `gguf` case.

**Step 3: Update nomic-embed recipe**

Change `type: gguf` to `type: gguf-embed` in `host-cli/internal/catalog/embedded/recipes/models/nomic-embed-text-v1.5.yaml` and `recipes/models/nomic-embed-text-v1.5.yaml`.

**Step 4: Build and test**

Run: `cd host-cli && go build ./... && go test ./internal/toolkit/ -v`
Expected: PASS. The chat menu no longer needs to filter embedding models — they're a different type.

**Step 5: Commit**

```
git commit -m "feat: add gguf-embed type with models/embed/ directory, remove filename heuristic"
```

---

### Task 2: Simplify FindEmbeddingModel to scan `models/embed/`

**Files:**
- Modify: `host-cli/internal/embedder/embedder.go:185-216` (FindEmbeddingModel)
- Test: `host-cli/internal/embedder/embedder_test.go`

**Step 1: Write test**

```go
func TestFindEmbeddingModel(t *testing.T) {
	dir := t.TempDir()
	embedDir := filepath.Join(dir, "models", "embed")
	os.MkdirAll(embedDir, 0o755)

	// No model — should error.
	_, err := FindEmbeddingModel(dir)
	if err == nil {
		t.Fatal("expected error when no model present")
	}

	// Add a model.
	modelPath := filepath.Join(embedDir, "all-MiniLM-L6-v2-Q8_0.gguf")
	os.WriteFile(modelPath, []byte("fake"), 0o644)

	found, err := FindEmbeddingModel(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != modelPath {
		t.Errorf("found = %q, want %q", found, modelPath)
	}
}
```

**Step 2: Rewrite FindEmbeddingModel**

```go
// FindEmbeddingModel returns the path to the first .gguf file in models/embed/.
func FindEmbeddingModel(driveRoot string) (string, error) {
	embedDir := filepath.Join(driveRoot, "models", "embed")
	matches, err := filepath.Glob(filepath.Join(embedDir, "*.gguf"))
	if err != nil {
		return "", fmt.Errorf("embedder: glob models/embed: %w", err)
	}
	for _, m := range matches {
		if !strings.HasPrefix(filepath.Base(m), "._") {
			return m, nil
		}
	}
	return "", fmt.Errorf("embedder: no embedding model found in %s", embedDir)
}
```

**Step 3: Run tests**

Run: `cd host-cli && go test ./internal/embedder/ -v`
Expected: PASS

**Step 4: Commit**

```
git commit -m "refactor(embedder): scan models/embed/ directory instead of pattern-matching filenames"
```

---

### Task 3: Simplify drive-runtime `findEmbeddingModel` the same way

**Files:**
- Modify: `drive-runtime/internal/search/search.go:252-268` (findEmbeddingModel)

**Step 1: Rewrite**

```go
func findEmbeddingModel(driveRoot string) string {
	matches, _ := filepath.Glob(filepath.Join(driveRoot, "models", "embed", "*.gguf"))
	for _, m := range matches {
		if !strings.HasPrefix(filepath.Base(m), "._") {
			return m
		}
	}
	return ""
}
```

**Step 2: Build and test**

Run: `cd drive-runtime && go build ./... && go test ./internal/search/ -v`
Expected: PASS

**Step 3: Commit**

```
git commit -m "refactor(search): scan models/embed/ directory instead of pattern-matching filenames"
```

---

### Task 4: Record embedding model in search.db during indexing

**Files:**
- Modify: `host-cli/internal/commands/index_semantic.go:83-106`
- Modify: `host-cli/internal/searchdb/db.go` (add EmbeddingDims)
- Test: `host-cli/internal/searchdb/db_test.go`

**Step 1: Write test for EmbeddingDims**

In `host-cli/internal/searchdb/db_test.go`:

```go
func TestEmbeddingDims(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid, _ := db.UpsertSource("test.zim", "Test")
	db.InsertArticles(sid, []Article{{Path: "/A/One", Title: "One", Body: "body"}})

	// 384-dim embedding = 384 * 4 bytes = 1536 bytes.
	vec := make([]byte, 384*4)
	db.InsertEmbeddings([]EmbeddingPair{{ArticleID: 1, Vector: vec}})

	dims, err := db.EmbeddingDims()
	if err != nil {
		t.Fatal(err)
	}
	if dims != 384 {
		t.Errorf("dims = %d, want 384", dims)
	}
}
```

**Step 2: Add EmbeddingDims to searchdb**

```go
// EmbeddingDims returns the dimension count from the first stored vector (4 bytes per float32).
func (d *DB) EmbeddingDims() (int, error) {
	var blobLen int
	err := d.db.QueryRow("SELECT length(vector) FROM embeddings LIMIT 1").Scan(&blobLen)
	if err != nil {
		return 0, err
	}
	return blobLen / 4, nil
}
```

**Step 3: Update indexer to record model info**

In `host-cli/internal/commands/index_semantic.go`, after finding the model (line 84), add model ID derivation and change detection:

```go
modelPath, err := embedder.FindEmbeddingModel(root)
if err != nil {
	return err
}

// Derive stable model ID from filename.
modelID := strings.TrimSuffix(filepath.Base(modelPath), filepath.Ext(modelPath))

// Detect model change — force re-embedding if model switched.
previousModel, _ := db.GetMeta("embedding_model")
if previousModel != "" && previousModel != modelID && !force {
	notify(SemanticProgress{Status: "starting", Detail: fmt.Sprintf("Model changed (%s → %s), re-embedding...", previousModel, modelID)})
	if err := db.DeleteAllEmbeddings(); err != nil {
		return fmt.Errorf("clearing old embeddings: %w", err)
	}
}
```

After the embedding loop (around line 104), record the model:

```go
if err := db.SetMeta("embedding_model", modelID); err != nil {
	return fmt.Errorf("setting embedding_model: %w", err)
}
if dimCount, err := db.EmbeddingDims(); err == nil && dimCount > 0 {
	db.SetMeta("embedding_dims", fmt.Sprintf("%d", dimCount))
}
```

**Step 4: Run tests**

Run: `cd host-cli && go test ./internal/searchdb/ ./internal/commands/ -v`
Expected: PASS

**Step 5: Commit**

```
git commit -m "feat(index): record embedding model ID and dims in search.db, auto-detect model change"
```

---

### Task 5: Drive-runtime reads model ID from search.db

**Files:**
- Modify: `drive-runtime/internal/search/search.go:34-39` (Capabilities)
- Modify: `drive-runtime/internal/search/search.go:229-250` (detectCapabilities)
- Modify: `drive-runtime/internal/search/session.go:18-23` (SessionInfo)
- Modify: `drive-runtime/internal/search/session.go:46-82` (NewSession)

**Step 1: Add EmbeddingModelID to Capabilities**

```go
type Capabilities struct {
	HasEmbeddings    bool
	HasEmbeddingData bool
	HasLlamaServer   bool
	EmbeddingModel   string // file path from models/embed/
	EmbeddingModelID string // model ID from search.db meta
}
```

**Step 2: Read model ID in detectCapabilities**

Add to the batched SQL query (append a 5th query):

```sql
SELECT COALESCE((SELECT value FROM meta WHERE key='embedding_model'), '');
```

Parse the 5th line into `caps.EmbeddingModelID`.

**Step 3: Validate in NewSession**

After `detectCapabilities`, if model ID is recorded but no model file found:

```go
if caps.EmbeddingModelID != "" && caps.EmbeddingModel == "" {
	caps.HasEmbeddingData = false
}
```

Add `EmbeddingModelID` to `SessionInfo`.

**Step 4: Build and test**

Run: `cd drive-runtime && go build ./... && go test ./internal/search/ -v`
Expected: PASS

**Step 5: Commit**

```
git commit -m "feat(search): validate embedding model presence from search.db metadata"
```

---

### Task 6: Add all-MiniLM-L6-v2 recipe and update presets

**Files:**
- Create: `host-cli/internal/catalog/embedded/recipes/models/all-minilm-l6-v2.yaml`
- Create: `recipes/models/all-minilm-l6-v2.yaml`
- Modify: `host-cli/internal/catalog/embedded/presets/test-runtime-2gb.yaml`
- Modify: `host-cli/internal/catalog/embedded/presets/test-1gb.yaml`

**Step 1: Create recipe**

```yaml
id: all-minilm-l6-v2
type: gguf-embed
display_group: models
tags: [embedding, search]
depth: reference-only
size_gb: 0.025
url: https://huggingface.co/second-state/All-MiniLM-L6-v2-Embedding-GGUF/resolve/main/all-MiniLM-L6-v2-Q8_0.gguf
description: all-MiniLM-L6-v2 (Q8) — lightweight English embedding model for search
license:
  id: Apache-2.0
  attribution: Microsoft / sentence-transformers
```

**Step 2: Swap default in presets**

Replace `nomic-embed-text-v1.5` with `all-minilm-l6-v2` in preset YAMLs.

**Step 3: Run catalog tests**

Run: `cd host-cli && go test ./internal/catalog/ -v`
Expected: PASS

**Step 4: Commit**

```
git commit -m "feat(presets): add all-MiniLM-L6-v2 recipe (25MB), make default embedding model"
```

---

### Task 7: Cleanup — remove dead code

**Files:**
- Modify: `host-cli/internal/toolkit/toolkit.go` — verify `isEmbeddingModel` is deleted (from task 1)
- Modify: `drive-runtime/internal/search/search.go` — old `findEmbeddingModel` glob patterns removed (from task 3)
- Modify: `host-cli/internal/embedder/embedder.go` — old nomic-specific pattern matching removed (from task 2)
- Modify: `host-cli/internal/catalog/embedded/recipes/models/nomic-embed-text-v1.5.yaml` — verify type is `gguf-embed`
- Modify: `drive-runtime/internal/chat/chat.go` — verify the `isEmbeddingModel` filter in `ResolveModel` still works (chat models are `gguf` type, so they're in `models/`, not `models/embed/` — the filter is now unnecessary but harmless)

**Step 1: Grep for dead references**

```bash
grep -rn "isEmbeddingModel\|nomic.*embed\|findEmbeddingModel.*nomic" host-cli/ drive-runtime/
```

Remove any stale references.

**Step 2: Verify chat model resolution still works**

The `isEmbeddingModel` filter in `drive-runtime/internal/chat/chat.go:39-42` filtered embedding models from the chat model list by filename pattern. With embedding models now in `models/embed/` instead of `models/`, the chat glob (`models/*.gguf`) won't match them anyway. Remove the filter.

**Step 3: Build and test everything**

```bash
cd host-cli && go build ./... && go test ./...
cd ../drive-runtime && go build ./... && go test ./...
```

**Step 4: Commit**

```
git commit -m "chore: remove dead embedding model filename heuristics"
```

---

### Task 8: End-to-end verification

**Step 1: Rebuild**

```bash
./scripts/build-drive-runtime.sh
cd host-cli && go build -o svalbard ./cmd/svalbard
```

**Step 2: Apply and index**

```bash
./svalbard --vault /tmp/svalbard-test apply
./svalbard --vault /tmp/svalbard-test index
./svalbard --vault /tmp/svalbard-test index --semantic
```

**Step 3: Verify**

```bash
# Model in correct directory
ls /tmp/svalbard-test/models/embed/

# Metadata recorded
sqlite3 /tmp/svalbard-test/data/search.db "SELECT * FROM meta;"

# Search works
/tmp/svalbard-test/run
```

**Step 4: Verify model swap detection**

```bash
# Copy a different model, re-index, confirm re-embedding triggered
```
