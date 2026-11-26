package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/htmlpack"
)

const htmlUsage = `  %[1]s html pack <page.html> [-o out.html] [--dir]

pack bundles the page's module scripts with the drive's JS libraries (bare
imports such as "chart.js/auto"), the files they import (CSV, JSON, GeoJSON),
stylesheets, fonts and images into one HTML file that opens by double-click
without a network. It refuses a page that loads from the network, fetches a
file beside it, or uses a library that needs the drive's server; --dir writes
a folder (index.html and its files) for the drive's server instead.
`

// runHTML is the html subcommand.
func runHTML(ctx context.Context, stdout, stderr io.Writer, driveRoot string, args []string) error {
	usage := func(problem string) error {
		fmt.Fprintf(stderr, htmlUsage, shellWord(skillsProgram))
		return errors.New(problem)
	}
	if len(args) == 0 || args[0] != "pack" {
		if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
			fmt.Fprintf(stdout, htmlUsage, shellWord(skillsProgram))
			return nil
		}
		return usage("html needs a subcommand: pack")
	}
	opts := htmlpack.Options{DriveRoot: driveRoot}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		switch arg := rest[i]; {
		case arg == "-o" || arg == "--out":
			if i+1 >= len(rest) {
				return usage(arg + " needs a file name")
			}
			i++
			opts.Out = rest[i]
		case strings.HasPrefix(arg, "--out="):
			opts.Out = strings.TrimPrefix(arg, "--out=")
		case arg == "--dir":
			opts.Dir = true
		case strings.HasPrefix(arg, "-") && arg != "-":
			return usage("unknown option " + arg)
		case opts.Page == "":
			opts.Page = arg
		default:
			return usage("pack takes one page; " + arg + " is a second")
		}
	}
	if opts.Page == "" {
		return usage("pack needs the page to pack")
	}
	result, err := htmlpack.Pack(ctx, opts)
	var problems *htmlpack.ProblemsError
	if errors.As(err, &problems) {
		fmt.Fprintf(stderr, "%s cannot be packed into a page that works offline:\n", filepath.Base(opts.Page))
		for _, p := range problems.Problems {
			fmt.Fprintf(stderr, "  %s\n", p)
		}
		return fmt.Errorf("nothing written (%d problems)", len(problems.Problems))
	}
	if err != nil {
		return err
	}
	out := result.Out
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, out); err == nil && !strings.HasPrefix(rel, "..") {
			out = rel
		}
	}
	fmt.Fprintf(stdout, "wrote %s (%s, %s)\n", out, humanSize(result.Bytes), result.Portability)
	if result.Portability == "directory" {
		fmt.Fprintln(stdout, "open it through the drive's server, not by double-click")
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(stdout, "warning: %s\n", w)
	}
	return nil
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}
