package actions_test

import (
	"reflect"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/actions"
)

func TestNativeInvocationRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   actions.NativeInvocation
		want actions.NativeInvocation
	}{
		{name: "inspect", in: invocation("inspect", nil)},
		{name: "verify", in: invocation("verify", nil)},
		{name: "share", in: invocation("share", nil)},
		{name: "browse", in: invocation("browse", map[string]string{"zim": "wiki.zim"})},
		{name: "browse without archive", in: invocation("browse", nil)},
		{name: "apps", in: invocation("apps", map[string]string{"app": "sqliteviz"})},
		{name: "maps", in: invocation("maps", nil)},
		{name: "chat", in: invocation("chat", map[string]string{"model": "gemma.gguf"})},
		{name: "chat without model", in: invocation("chat", nil)},
		{name: "agent with model", in: invocation("agent", map[string]string{"client": "opencode", "model": "gemma.gguf"})},
		{name: "agent without model", in: invocation("agent", map[string]string{"client": "opencode"})},
		{name: "serve all default bind", in: invocation("serve-all", nil), want: invocation("serve-all", map[string]string{"bind": "127.0.0.1"})},
		{name: "serve all custom bind", in: invocation("serve-all", map[string]string{"bind": "0.0.0.0"})},
		{name: "search", in: invocation("search", map[string]string{"query": "water"})},
		{name: "search without query", in: invocation("search", nil)},
		{name: "embedded shell", in: invocation("embedded-shell", nil)},
		{name: "activate shell", in: invocation("activate-shell", nil)},
		{name: "mcp serve", in: invocation("mcp-serve", nil)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			argv, err := actions.EncodeNativeInvocation(tc.in)
			if err != nil {
				t.Fatalf("EncodeNativeInvocation() error = %v", err)
			}
			got, known, err := actions.DecodeNativeInvocation(argv)
			if err != nil {
				t.Fatalf("DecodeNativeInvocation(%v) error = %v", argv, err)
			}
			if !known {
				t.Fatalf("DecodeNativeInvocation(%v) known = false", argv)
			}
			want := tc.want
			if want.ActionID == "" {
				want = tc.in
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("round trip = %#v, want %#v", got, want)
			}
		})
	}
}

func TestNativeInvocationValidation(t *testing.T) {
	tests := []struct {
		name string
		run  func() (bool, error)
	}{
		{
			name: "unknown action ID",
			run: func() (bool, error) {
				_, err := actions.EncodeNativeInvocation(invocation("unknown", nil))
				return true, err
			},
		},
		{
			name: "unknown hidden subcommand",
			run: func() (bool, error) {
				_, known, err := actions.DecodeNativeInvocation([]string{"__native-unknown"})
				return known, err
			},
		},
		{
			name: "missing required argument",
			run: func() (bool, error) {
				_, known, err := actions.DecodeNativeInvocation([]string{actions.NativeAppsSubcommand})
				return known, err
			},
		},
		{
			name: "extra positional argument",
			run: func() (bool, error) {
				_, known, err := actions.DecodeNativeInvocation([]string{actions.NativeAgentSubcommand, "opencode", "model", "extra"})
				return known, err
			},
		},
		{
			name: "unknown named argument",
			run: func() (bool, error) {
				_, err := actions.EncodeNativeInvocation(invocation("search", map[string]string{"invalid": "value"}))
				return true, err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			known, err := tc.run()
			if !known {
				t.Fatal("native command was not recognized")
			}
			if err == nil {
				t.Fatal("validation error = nil")
			}
		})
	}
}

func invocation(actionID string, args map[string]string) actions.NativeInvocation {
	if args == nil {
		args = map[string]string{}
	}
	return actions.NativeInvocation{ActionID: actionID, Args: args}
}
