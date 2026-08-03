package commands

import (
	"strings"
	"testing"
)

func TestDefaultZimName(t *testing.T) {
	cases := []struct {
		url     string
		want    string
		wantErr bool
	}{
		{url: "https://opensourcelowtech.org/", want: "opensourcelowtech"},
		{url: "https://www.example.com/path", want: "example"},
		{url: "http://docs.example.com", want: "docs"},
		{url: "not a url", wantErr: true},
		{url: "", wantErr: true},
	}
	for _, c := range cases {
		got, err := DefaultZimName(c.url)
		if c.wantErr {
			if err == nil {
				t.Errorf("DefaultZimName(%q): expected error, got %q", c.url, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("DefaultZimName(%q): %v", c.url, err)
			continue
		}
		if got != c.want {
			t.Errorf("DefaultZimName(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestZimRecipeShape(t *testing.T) {
	item := zimRecipe("lowtech", "https://opensourcelowtech.org/")

	if item.ID != "lowtech" || item.Type != "zim" || item.Strategy != "build" {
		t.Fatalf("unexpected item identity: %+v", item)
	}
	if item.Build == nil || len(item.Build.Steps) != 2 {
		t.Fatalf("expected 2 build steps, got %+v", item.Build)
	}
	exec := item.Build.Steps[0]
	if exec.Exec != "zimit" || exec.DockerImage != "ghcr.io/openzim/zimit:latest" {
		t.Errorf("unexpected exec step: %+v", exec)
	}
	joined := strings.Join(exec.Args, " ")
	for _, want := range []string{"{source_url}", "--zim-file", "lowtech.zim", "{vault}/zim"} {
		if !strings.Contains(joined, want) {
			t.Errorf("exec args missing %q: %s", want, joined)
		}
	}
	verify := item.Build.Steps[1]
	if verify.Verify != "{output}" || verify.MinSize < 1 {
		t.Errorf("unexpected verify step: %+v", verify)
	}
	if item.Build.Output != "lowtech.zim" {
		t.Errorf("Output = %q, want lowtech.zim", item.Build.Output)
	}
}
