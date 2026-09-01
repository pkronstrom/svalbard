package builder

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/ncruces/go-sqlite3/driver"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

type fimeaCode struct {
	Value string `xml:"value,attr"`
}

type fimeaProduct struct {
	ID         string    `xml:"id,attr"`
	Name       string    `xml:"Kauppanimi"`
	ATC        fimeaCode `xml:"ATC-koodi"`
	Strength   string    `xml:"Vahvuus"`
	DosageForm fimeaCode `xml:"Laakemuoto"`
}

type fimeaSubstanceRef struct {
	Ref string `xml:"Laakeaine-ref,attr"`
}

type fimeaPackage struct {
	ProductRef string              `xml:"Laakevalmiste-ref,attr"`
	PackageID  string              `xml:"Pakkaustunnus"`
	VNR        string              `xml:"VNR-numero"`
	Size       string              `xml:"Pakkauskokoteksti"`
	RxStatus   fimeaCode           `xml:"Reseptistatus"`
	Substances []fimeaSubstanceRef `xml:"Pakkaus_Laakeaine"`
}

type fimeaActiveSubstance struct {
	Name fimeaCode `xml:"Aine"`
}

type fimeaSubstance struct {
	ID      string                 `xml:"id,attr"`
	Actives []fimeaActiveSubstance `xml:"VaikuttavaAine"`
}

type fimeaRegistry struct {
	Products   []fimeaProduct   `xml:"Laakevalmiste"`
	Packages   []fimeaPackage   `xml:"Pakkaus"`
	Substances []fimeaSubstance `xml:"Laakeaine"`
}

func buildReferenceStatic(root string, recipe catalog.Item, _ *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error) {
	if recipe.Build == nil || recipe.Build.SourceURL == "" {
		return nil, fmt.Errorf("reference-static %s: source_url required", recipe.ID)
	}
	if recipe.Build.Config["source_format"] != "xml" {
		return nil, fmt.Errorf("reference-static %s: unsupported source_format %q", recipe.ID, recipe.Build.Config["source_format"])
	}
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	workdir := filepath.Join(root, ".staging", "build", recipe.ID)
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return nil, err
	}
	source := filepath.Join(workdir, "source.xml")
	emit(opts, BuildEvent{RecipeID: recipe.ID, Procedure: "download", State: EventStarted, Message: "downloading reference XML"})
	if err := stepDownload(ctx, recipe.Build.SourceURL, source); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}
	var registry fimeaRegistry
	if err := xml.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("reference-static %s: parse XML: %w", recipe.ID, err)
	}
	output := filepath.Join(root, toolkit.TypeDirs[recipe.Type], recipe.ID+".sqlite")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, err
	}
	if err := writeFimeaSQLite(output, registry); err != nil {
		return nil, err
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, err
	}
	checksum, _ := downloader.ComputeSHA256(output)
	return []manifest.RealizedEntry{{
		ID: recipe.ID, Type: recipe.Type, Filename: filepath.Base(output),
		RelativePath: filepath.Join(toolkit.TypeDirs[recipe.Type], filepath.Base(output)),
		SizeBytes:    info.Size(), ChecksumSHA256: checksum, SourceStrategy: "build",
	}}, nil
}

func writeFimeaSQLite(path string, registry fimeaRegistry) error {
	_ = os.Remove(path)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE medicines (
			package_id TEXT PRIMARY KEY, vnr TEXT, name TEXT NOT NULL,
			active_ingredient TEXT, atc_code TEXT, strength TEXT,
			dosage_form TEXT, package_size TEXT, prescription_status TEXT
		);
		CREATE VIRTUAL TABLE medicines_fts USING fts5(
			name, active_ingredient, atc_code, content='medicines', content_rowid='rowid'
		);`); err != nil {
		return err
	}
	products := make(map[string]fimeaProduct, len(registry.Products))
	for _, product := range registry.Products {
		products[product.ID] = product
	}
	substances := make(map[string]string, len(registry.Substances))
	for _, substance := range registry.Substances {
		var names []string
		for _, active := range substance.Actives {
			if active.Name.Value != "" {
				names = append(names, active.Name.Value)
			}
		}
		substances[substance.ID] = strings.Join(names, ", ")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	insert, err := tx.Prepare(`INSERT INTO medicines VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, pkg := range registry.Packages {
		product, ok := products[pkg.ProductRef]
		if !ok || product.Name == "" || pkg.PackageID == "" {
			continue
		}
		var names []string
		for _, reference := range pkg.Substances {
			if name := substances[reference.Ref]; name != "" {
				names = append(names, name)
			}
		}
		if _, err := insert.Exec(pkg.PackageID, pkg.VNR, product.Name, strings.Join(names, ", "), product.ATC.Value, product.Strength, product.DosageForm.Value, pkg.Size, pkg.RxStatus.Value); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO medicines_fts(rowid, name, active_ingredient, atc_code)
		SELECT rowid, name, active_ingredient, atc_code FROM medicines`); err != nil {
		return err
	}
	return tx.Commit()
}
