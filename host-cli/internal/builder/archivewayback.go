package builder

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Wayback Machine endpoints; tests point them at local servers.
var (
	archiveWaybackAPI  = "https://archive.org/wayback/available"
	archiveWaybackHost = "https://web.archive.org"
)

// archiveWaybackArticle reads the closest Wayback Machine capture of a page
// that could not be fetched live. Links resolve against the original address;
// images and files that fail live are retried through the same capture. A
// recent capture can itself be a bot check, so an older one is tried next.
func archiveWaybackArticle(fetcher *archiveFetcher, source *url.URL, rule *compiledArchiveRule) (*archiveArticle, error) {
	var lastErr error
	for _, near := range []string{"", archiveWaybackOlder} {
		timestamp, err := archiveWaybackTimestamp(fetcher, source, near)
		if err != nil {
			return nil, err
		}
		body, contentType, _, err := fetcher.get(archiveWaybackURL(timestamp, "id_", source), maxArchiveResponseBytes)
		if err != nil {
			return nil, err
		}
		article, err := archiveParseHTML(fetcher, body, contentType, source, rule)
		if err == nil {
			article.Wayback = timestamp
			return article, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// archiveWaybackOlder is the date the second lookup asks for.
const archiveWaybackOlder = "2023"

func archiveWaybackTimestamp(fetcher *archiveFetcher, source *url.URL, near string) (string, error) {
	endpoint, err := url.Parse(archiveWaybackAPI)
	if err != nil {
		return "", err
	}
	query := url.Values{"url": {source.String()}}
	if near != "" {
		query.Set("timestamp", near)
	}
	endpoint.RawQuery = query.Encode()
	body, _, _, err := fetcher.get(endpoint, 1<<20)
	if err != nil {
		return "", err
	}
	var reply struct {
		Snapshots struct {
			Closest struct {
				Available bool   `json:"available"`
				Status    string `json:"status"`
				Timestamp string `json:"timestamp"`
			} `json:"closest"`
		} `json:"archived_snapshots"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return "", fmt.Errorf("wayback lookup %s: %w", source, err)
	}
	closest := reply.Snapshots.Closest
	if !closest.Available || closest.Status != "200" || closest.Timestamp == "" {
		return "", fmt.Errorf("no Wayback Machine capture of %s", source)
	}
	return closest.Timestamp, nil
}

// archiveWaybackURL addresses a capture in its raw form: mode "id_" for pages
// and files, "im_" for images.
func archiveWaybackURL(timestamp, mode string, target *url.URL) *url.URL {
	address, err := url.Parse(archiveWaybackHost + "/web/" + timestamp + mode + "/" + target.String())
	if err != nil {
		return target
	}
	return address
}

// archiveWaybackDate formats a capture timestamp (YYYYMMDDhhmmss) as a date.
func archiveWaybackDate(timestamp string) string {
	if len(timestamp) < 8 {
		return timestamp
	}
	return timestamp[:4] + "-" + timestamp[4:6] + "-" + timestamp[6:8]
}
