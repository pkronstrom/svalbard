// js-lock writes the pinned bundle of a js-lib recipe from a small spec.
//
//	js-lock -spec recipes/js/js-core.lock.json -out recipes/js/js-core.yaml
//
// It resolves the spec's packages and their dependencies with the npm
// registry's metadata (no npm binary), downloads each tarball, checks it
// against the registry's sha512 integrity, and lays the packages out as an
// npm lockfile would (node_modules/<name>, nested where versions differ).
// It then bundles each library's entries with esbuild to find the files a
// page that imports them reads, and keeps only those: a library's bundle
// source includes that closure, its package.json, licence and README, and a
// dependency nothing imports is dropped. A library with a self-contained
// build ("umd" in the spec) is that one file and no dependencies.
//
// The recipe's bundle, archive size and each library's version are written;
// everything else in an existing recipe (description, licence, the other
// library fields) is kept. Nothing is written unless every step succeeds.
//
// The spec:
//
//	{
//	  "id": "js-core",
//	  "packages": {"chart.js": "4.x", "d3": "7.x"},
//	  "entries": {"chart.js": ["chart.js/auto"]},
//	  "include": {"chart.js": ["dist/helpers.js"]},
//	  "umd": {"d3": "dist/d3.min.js"},
//	  "tarballs": {"xlsx": {"url": "https://cdn.sheetjs.com/xlsx-0.20.3/xlsx-0.20.3.tgz", "version": "0.20.3", "sha256": ""}}
//	}
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	specPath := flag.String("spec", "", "lock spec (recipes/js/<id>.lock.json)")
	outPath := flag.String("out", "", "recipe to write (recipes/js/<id>.yaml)")
	registry := flag.String("registry", "https://registry.npmjs.org", "npm registry")
	cache := flag.String("cache", "", "folder for downloaded tarballs (default: the user cache)")
	flag.Parse()
	if *specPath == "" || *outPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *cache == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			log.Fatal(err)
		}
		*cache = filepath.Join(dir, "svalbard", "js-lock")
	}
	locker := &locker{registry: *registry, cache: *cache, client: &http.Client{Timeout: 5 * time.Minute}, log: os.Stderr}
	if err := locker.run(context.Background(), *specPath, *outPath); err != nil {
		fmt.Fprintln(os.Stderr, "js-lock:", err)
		os.Exit(1)
	}
}
