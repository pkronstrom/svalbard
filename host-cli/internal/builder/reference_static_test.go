package builder_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"

	"github.com/pkronstrom/svalbard/host-cli/internal/builder/buildertest"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestReferenceStaticBuildsSearchableFimeaDatabase(t *testing.T) {
	xml := []byte(`<Perusrekisteri>
  <Pakkaus Laakevalmiste-ref="lv1">
    <Pakkaustunnus>pkg1</Pakkaustunnus><VNR-numero>12345</VNR-numero>
    <Pakkauskokoteksti>20 tablettia</Pakkauskokoteksti>
    <Reseptistatus value="R"/>
    <Pakkaus_Laakeaine Laakeaine-ref="sub1"/>
  </Pakkaus>
  <Laakevalmiste id="lv1">
    <Kauppanimi>TestMed</Kauppanimi><ATC-koodi value="M01AE01"/>
    <Vahvuus>400 mg</Vahvuus><Laakemuoto value="tabletti"/>
  </Laakevalmiste>
  <Laakeaine id="sub1"><VaikuttavaAine><Aine value="ibuprofen"/></VaikuttavaAine></Laakeaine>
</Perusrekisteri>`)
	recipe := catalog.Item{
		ID: "fimea", Type: "sqlite", Strategy: "build",
		Build: &catalog.BuildSpec{Family: "reference-static", Config: map[string]string{"source_format": "xml"}},
	}
	scenario := buildertest.New(t, recipe)
	scenario.Recipe.Build.SourceURL = scenario.Source(xml)
	scenario.Build()
	if scenario.Err != nil {
		t.Fatal(scenario.Err)
	}

	db, err := sql.Open("sqlite3", filepath.Join(scenario.Root, "data", "fimea.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var name, ingredient, atc string
	if err := db.QueryRow(`SELECT name, active_ingredient, atc_code FROM medicines WHERE package_id='pkg1'`).Scan(&name, &ingredient, &atc); err != nil {
		t.Fatal(err)
	}
	if name != "TestMed" || ingredient != "ibuprofen" || atc != "M01AE01" {
		t.Fatalf("medicine = %q, %q, %q", name, ingredient, atc)
	}
	var matches int
	if err := db.QueryRow(`SELECT count(*) FROM medicines_fts WHERE medicines_fts MATCH 'ibuprofen'`).Scan(&matches); err != nil {
		t.Fatal(err)
	}
	if matches != 1 {
		t.Fatalf("FTS matches = %d", matches)
	}
}
