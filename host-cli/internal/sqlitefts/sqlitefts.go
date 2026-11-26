// Package sqlitefts registers the ncruces SQLite driver with FTS5 enabled.
// Since go-sqlite3 v0.35 FTS5 is compiled separately and must be registered
// on each connection; import this package for its side effect instead of the
// bare driver.
package sqlitefts

import (
	"github.com/ncruces/go-sqlite3"
	_ "github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/fts5"
)

func init() { sqlite3.AutoExtension(fts5.Register) }
