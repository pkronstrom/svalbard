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
// images and files that fail live are retried through the same capture.
func archiveWaybackArticle(fetcher *archiveFetcher, source *url.URL, rule *compiledArchiveRule) (*archiveArticle, error) {
	timestamp, err := archiveWaybackTimestamp(fetcher, source)
	if err != nil {
		return nil, err
	}
	body, contentType, _, err := fetcher.get(archiveWaybackURL(timestamp, "id_", source), maxArchiveResponseBytes)
	if err != nil {
		return nil, err
	}
	article, err := archiveParseHTML(fetcher, body, contentType, source, rule)
	if err != nil {
		return nil, err
	}
	article.Wayback = timestamp
	return article, nil
}

func archiveWaybackTimestamp(fetcher *archiveFetcher, source *url.URL) (string, error) {
	endpoint, err := url.Parse(archiveWaybackAPI)
	if err != nil {
		return "", err
	}
	endpoint.RawQuery = url.Values{"url": {source.String()}}.Encode()
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
