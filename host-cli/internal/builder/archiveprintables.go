package builder

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Printables endpoints; tests point them at local servers.
var (
	printablesAPI   = "https://api.printables.com/graphql/"
	printablesMedia = "https://media.printables.com/"
)

var printablesModelPath = regexp.MustCompile(`^/(?:[a-z]{2}/)?model/(\d+)`)

const printablesModelQuery = `query PrintProfile($id: ID!) { print(id: $id) {
	name summary description license { name } user { publicUsername }
	images { filePath } stls { id name fileSize } otherFiles { id name fileSize } } }`

const printablesDownloadQuery = `mutation GetDownloadLink($printId: ID!, $files: [DownloadFileInput!]!, $source: DownloadSourceEnum!) {
	getDownloadLink(printId: $printId, files: $files, source: $source) { ok output { files { id link } } } }`

type printablesFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	FileSize int    `json:"fileSize"`
}

// printablesArticle reads a model through the GraphQL API the Printables site
// uses; its pages only render with scripts behind a browser challenge.
func printablesArticle(fetcher *archiveFetcher, source *url.URL) (*archiveArticle, error) {
	match := printablesModelPath.FindStringSubmatch(source.Path)
	if match == nil {
		return archiveHTMLArticle(fetcher, source, nil)
	}
	var model struct {
		Print *struct {
			Name        string `json:"name"`
			Summary     string `json:"summary"`
			Description string `json:"description"`
			License     *struct {
				Name string `json:"name"`
			} `json:"license"`
			User *struct {
				PublicUsername string `json:"publicUsername"`
			} `json:"user"`
			Images []struct {
				FilePath string `json:"filePath"`
			} `json:"images"`
			Stls       []printablesFile `json:"stls"`
			OtherFiles []printablesFile `json:"otherFiles"`
		} `json:"print"`
	}
	if err := printablesQuery(fetcher, printablesModelQuery, map[string]any{"id": match[1]}, &model); err != nil {
		return nil, err
	}
	print := model.Print
	if print == nil {
		return nil, fmt.Errorf("printables model %s not found", match[1])
	}
	article := &archiveArticle{Title: print.Name, Base: source, Description: print.Summary}
	if print.User != nil {
		article.Author = print.User.PublicUsername
	}
	if print.License != nil {
		article.License = print.License.Name
	}
	var page strings.Builder
	for index, image := range print.Images {
		if index == 1 {
			page.WriteString(print.Description)
			page.WriteString("<h2>Pictures</h2>")
		}
		fmt.Fprintf(&page, `<img src="%s" alt="">`, template.HTMLEscapeString(printablesMedia+strings.TrimPrefix(image.FilePath, "/")))
	}
	if len(print.Images) < 2 {
		page.WriteString(print.Description)
	}
	article.Content = archiveContainer()
	nodes, err := html.ParseFragment(strings.NewReader(page.String()), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		article.Content.AppendChild(node)
	}
	archiveMarkVideoEmbeds(fetcher, article, article.Content)
	pruneArchiveElements(article.Content)
	article.Files = printablesFiles(fetcher, match[1], map[string][]printablesFile{"stl": print.Stls, "other": print.OtherFiles})
	return article, nil
}

// printablesFiles asks for signed download links, skipping files too large to
// keep; a refused request leaves the model without files.
func printablesFiles(fetcher *archiveFetcher, model string, groups map[string][]printablesFile) []archiveFile {
	names := make(map[string]string)
	requests := make([]map[string]any, 0, len(groups))
	for fileType, files := range groups {
		ids := make([]string, 0, len(files))
		for _, file := range files {
			if file.FileSize <= maxArchiveResponseBytes {
				ids = append(ids, file.ID)
				names[file.ID] = file.Name
			}
		}
		if len(ids) > 0 {
			requests = append(requests, map[string]any{"fileType": fileType, "ids": ids})
		}
	}
	if len(requests) == 0 {
		return nil
	}
	var reply struct {
		Link struct {
			Output *struct {
				Files []struct {
					ID   string `json:"id"`
					Link string `json:"link"`
				} `json:"files"`
			} `json:"output"`
		} `json:"getDownloadLink"`
	}
	variables := map[string]any{"printId": model, "files": requests, "source": "model_detail"}
	if err := printablesQuery(fetcher, printablesDownloadQuery, variables, &reply); err != nil || reply.Link.Output == nil {
		return nil
	}
	files := make([]archiveFile, 0, len(reply.Link.Output.Files))
	for _, file := range reply.Link.Output.Files {
		if file.Link != "" {
			name := names[file.ID]
			if name == "" {
				name = file.Link
			}
			files = append(files, archiveFile{Name: name, URL: file.Link})
		}
	}
	return files
}

func printablesQuery(fetcher *archiveFetcher, query string, variables map[string]any, result any) error {
	endpoint, err := url.Parse(printablesAPI)
	if err != nil {
		return err
	}
	request, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	origin := (&url.URL{Scheme: "https", Host: "www.printables.com"}).String()
	header := http.Header{"Content-Type": {"application/json"}, "Origin": {origin}, "Referer": {origin + "/"}}
	body, _, _, err := fetcher.request(http.MethodPost, endpoint, request, header, maxArchiveResponseBytes)
	if err != nil {
		return err
	}
	var reply struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return fmt.Errorf("printables API: %w", err)
	}
	if len(reply.Errors) > 0 {
		return fmt.Errorf("printables API: %s", reply.Errors[0].Message)
	}
	return json.Unmarshal(reply.Data, result)
}
