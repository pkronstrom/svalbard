package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
)

func (c *SearchCapability) readDocumentOrVideo(ctx context.Context, source, path string, offset int) (ActionResult, error) {
	_, video := engine.VideoTime(path)
	if !video || !engine.IsEmbeddedSource(source) {
		return c.readDocument(ctx, source, path, offset)
	}
	path = strings.TrimPrefix(path, "/")
	text, err := c.indexedBody(source, path)
	if err != nil {
		return ActionResult{}, err
	}
	body, next, total := pageSlice(text, offset, c.read.Chars)
	if offset > 0 && offset >= total {
		return ActionResult{}, fmt.Errorf("offset %d is past the end of the transcript passage (%d characters)", offset, total)
	}
	result := readResult{Source: source, Path: path, Title: filepath.Base(source), Body: body}
	if next < total {
		result.Continues = fmt.Sprintf("Call search_read with the same source and path and offset %d for the next part.", next)
	}
	return ActionResult{Data: result}, nil
}
