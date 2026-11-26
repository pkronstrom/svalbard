package settings

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

const usage = `usage:
  ./run settings                  list every setting
  ./run settings <key>            print one value
  ./run settings <key> <value>    change it
  ./run settings --reset <key>    return it to the default`

// Run is the `./run settings` command. It prints plain text only, so it can be
// piped and read by scripts, and touches nothing but the schema and the values
// file: no menu, no terminal queries.
func Run(stdout, stderr io.Writer, driveRoot string, args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Fprintln(stdout, usage)
		return nil
	}
	store, err := Open(driveRoot)
	if err != nil {
		return err
	}
	switch {
	case len(args) == 0:
		list(stdout, store)
		for _, problem := range store.Problems() {
			fmt.Fprintln(stderr, "warning: "+problem)
		}
		return nil
	case args[0] == "--reset":
		if len(args) != 2 {
			return errors.New("--reset takes one key: ./run settings --reset <key>")
		}
		if err := store.Reset(args[1]); err != nil {
			return err
		}
		setting, _ := store.Lookup(args[1])
		fmt.Fprintf(stdout, "%s = %s (default)\n", args[1], shown(setting, store.Value(args[1])))
		return nil
	case len(args) == 1:
		setting, ok := store.Lookup(args[0])
		if !ok {
			return store.unknownKey(args[0])
		}
		if problem := store.Problem(args[0]); problem != "" {
			fmt.Fprintln(stderr, "warning: "+problem)
		}
		// The explicit single-key form is the one place a secret is printed.
		fmt.Fprintln(stdout, setting.Format(store.Value(args[0])))
		return nil
	case len(args) == 2:
		if err := store.Set(args[0], args[1]); err != nil {
			return err
		}
		setting, _ := store.Lookup(args[0])
		fmt.Fprintf(stdout, "%s = %s\n", args[0], shown(setting, store.Value(args[0])))
		return nil
	default:
		return fmt.Errorf("too many arguments\n%s", usage)
	}
}

// list prints every setting under its group, with a mark on the ones changed
// from the default.
func list(w io.Writer, store *Store) {
	width := 0
	for _, key := range store.Keys() {
		width = max(width, len(key))
	}
	anyChanged := false
	for i, group := range store.Groups() {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, group.Label)
		for _, setting := range group.Settings {
			key := setting.Key()
			mark := " "
			if store.Changed(key) {
				mark = "*"
				anyChanged = true
			}
			line := fmt.Sprintf("  %-*s  %s %s", width, key, mark, shown(setting, store.Value(key)))
			fmt.Fprintln(w, strings.TrimRight(line, " "))
		}
	}
	fmt.Fprintln(w)
	if anyChanged {
		fmt.Fprintln(w, "* changed from the default")
	}
	fmt.Fprintln(w, "Change one with ./run settings <key> <value>, undo with ./run settings --reset <key>.")
}

// shown is Format for output that names the setting beside the value, where a
// secret is not revealed.
func shown(setting Setting, value any) string {
	if setting.Type == TypeSecret {
		if value == "" {
			return "not set"
		}
		return "set"
	}
	return setting.Format(value)
}
