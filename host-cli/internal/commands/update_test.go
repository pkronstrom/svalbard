package commands

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

// agedVault is a vault opened long after it was built: one item is current,
// one recipe changed, one was installed before tracking, one left the
// catalog, one never arrived and left the catalog too, and one is a local
// import.
func agedVault(t *testing.T, cat *catalog.Catalog) manifest.Manifest {
	t.Helper()
	ifixit, _ := cat.RecipeByID("ifixit")
	m := manifest.New("aged")
	m.Desired.Items = []string{"ifixit", "wikipedia-en-nopic", "withdrawn-zim", "never-arrived", "local:manual"}
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "ifixit", Type: "zim", RelativePath: "zim/ifixit.zim", AcquisitionHash: ifixit.AcquisitionHash()},
		{ID: "wikipedia-en-nopic", Type: "zim", RelativePath: "zim/wp.zim", AcquisitionHash: "acq-v1:sha256:an-older-recipe"},
		{ID: "withdrawn-zim", Type: "zim", RelativePath: "zim/withdrawn.zim", AcquisitionHash: "acq-v1:sha256:whatever"},
		{ID: "local:manual", Type: "pdf", RelativePath: "library/manual.pdf", SourceStrategy: "local"},
	}
	return m
}

func TestStatusNamesEachInstalledItemsVersionState(t *testing.T) {
	cat := catalog.NewTestCatalog(t)
	m := agedVault(t, cat)
	r := BuildStatus(cat, m)

	got := map[string]string{}
	for _, item := range r.Items {
		got[item.ID] = item.Status + "/" + item.Version
	}
	want := map[string]string{
		"ifixit":             "realized/" + VersionCurrent,
		"wikipedia-en-nopic": "realized/" + VersionUpdate,
		"withdrawn-zim":      "realized/" + VersionWithdrawn,
		"never-arrived":      "unavailable/",
		"local:manual":       "realized/" + VersionLocal,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("item states = %v\nwant %v", got, want)
	}
	if !reflect.DeepEqual(r.Updates, []string{"wikipedia-en-nopic"}) || !reflect.DeepEqual(r.Withdrawn, []string{"withdrawn-zim"}) || !reflect.DeepEqual(r.Unavailable, []string{"never-arrived"}) {
		t.Errorf("updates = %v, withdrawn = %v, unavailable = %v", r.Updates, r.Withdrawn, r.Unavailable)
	}

	// An entry from before hashes were recorded is unknown, not current.
	m.Realized.Entries[0].AcquisitionHash = ""
	if v := BuildStatus(cat, m).Items[0].Version; v != VersionUnknown {
		t.Errorf("entry without a hash: version = %q", v)
	}

	var text bytes.Buffer
	if err := WriteStatus(&text, cat, m); err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"update available", "withdrawn, kept", "1 update(s) available", "kept on this vault: withdrawn-zim", "svalbard remove"} {
		if !strings.Contains(text.String(), phrase) {
			t.Errorf("status text lacks %q:\n%s", phrase, text.String())
		}
	}
}

func TestPlanLeavesChangedAndWithdrawnItemsAlone(t *testing.T) {
	cat := catalog.NewTestCatalog(t)
	m := agedVault(t, cat)

	// Apply would touch neither the changed nor the withdrawn item.
	if p := planner.Build(m); len(p.ToRemove) != 0 || !reflect.DeepEqual(p.ToDownload, []string{"never-arrived"}) || len(p.ToUpdate) != 0 {
		t.Errorf("plan = %+v", p)
	}
	d := DescribePlan(cat, m)
	if len(d.Updates) != 1 || d.Updates[0].ID != "wikipedia-en-nopic" || d.Updates[0].Action != "update" {
		t.Errorf("plan updates = %+v", d.Updates)
	}
	var text bytes.Buffer
	if err := WritePlanDetail(&text, d); err != nil {
		t.Fatal(err)
	}
	WritePlanNext(&text, d)
	if !strings.Contains(text.String(), "svalbard update") {
		t.Errorf("plan text does not point at update:\n%s", text.String())
	}
}

func TestUpdateRefusesItemsItCannotReacquire(t *testing.T) {
	cat := catalog.NewTestCatalog(t)
	root := t.TempDir()
	m := agedVault(t, cat)
	if err := manifest.Save(root+"/manifest.yaml", m); err != nil {
		t.Fatal(err)
	}
	for id, phrase := range map[string]string{
		"withdrawn-zim": "its installed copy is kept",
		"local:manual":  "local import",
		"never-arrived": "not installed",
	} {
		err := UpdateVault(context.Background(), root, cat, []string{id}, false)
		if err == nil || !strings.Contains(err.Error(), phrase) {
			t.Errorf("update %s: err = %v, want it to say %q", id, err, phrase)
		}
	}
	saved, err := manifest.Load(root + "/manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.Realized.Entries, m.Realized.Entries) {
		t.Error("a refused update changed the manifest")
	}
}
