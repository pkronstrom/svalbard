package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type serviceEntry struct {
	Label, Icon, Service string
	Missing              bool
}

type serviceGroup struct{ Items []serviceEntry }
type inventory struct{ Groups []serviceGroup }

func (i inventory) localServices() []serviceEntry {
	var entries []serviceEntry
	seen := map[string]bool{}
	for _, group := range i.Groups {
		for _, item := range group.Items {
			switch item.Icon {
			case "browse", "documents", "maps", "apps", "wiki", "workspace", "chat":
				if item.Service != "" && !item.Missing && !seen[item.Service] {
					seen[item.Service] = true
					entries = append(entries, item)
				}
			}
		}
	}
	return entries
}

type registry struct {
	Version  string
	Draining bool
	Jobs     map[string]struct{ Status string }
	Records  []struct{ Status string }
}

func loopbackOrigin(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil ||
		(u.Path != "/" && u.Path != "") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("the drive did not return a dashboard address")
	}
	return "http://" + u.Host, nil
}

var localClient = &http.Client{
	Transport: &http.Transport{
		DialContext:       (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
		DisableKeepAlives: true,
	},
	Timeout:       20 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func dashboardRequest(origin, path string, payload, result any) error {
	validated, err := loopbackOrigin(origin)
	if err != nil {
		return err
	}
	method := http.MethodGet
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
		method = http.MethodPost
	}
	req, err := http.NewRequest(method, validated+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Origin", validated)
		req.Header.Set("X-Svalbard-Request", "1")
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := localClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("dashboard returned %d. %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(result)
	}
	return nil
}

type drive struct{ root, runtime string }

func (d drive) command(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, d.runtime, append([]string{"launch"}, args...)...)
	cmd.Dir = "/"
	cmd.Env = append(os.Environ(), "DRIVE_ROOT="+d.root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (d drive) address() (string, error) {
	raw, err := d.command("status")
	if err != nil {
		return "", err
	}
	if raw == "No managed dashboard is running for this vault." {
		return "", nil
	}
	return loopbackOrigin(raw)
}

func setService(origin, key string, active bool) error {
	route := "/api/services/" + url.PathEscape(key)
	if active {
		route += "/stop"
	}
	return dashboardRequest(origin, route, struct{}{}, nil)
}

func (d drive) prepareAndStop(checkHolders bool) error {
	origin, err := d.address()
	if err != nil || origin == "" {
		return err
	}
	var status registry
	if err := dashboardRequest(origin, "/api/registry", nil, &status); err != nil {
		return err
	}
	if err := dashboardRequest(origin, "/api/lifecycle", map[string]string{"action": "prepare", "version": status.Version}, nil); err != nil {
		return err
	}
	if !checkHolders {
		_, err := d.command("stop")
		return err
	}
	var holders struct {
		Users []struct {
			PID  int
			Name string
		}
	}
	if err := dashboardRequest(origin, "/api/drive-users", struct{}{}, &holders); err != nil {
		_ = dashboardRequest(origin, "/api/lifecycle", map[string]string{"action": "resume"}, nil)
		return err
	}
	if len(holders.Users) > 0 {
		_ = dashboardRequest(origin, "/api/lifecycle", map[string]string{"action": "resume"}, nil)
		return fmt.Errorf("%s (PID %d) is using the vault. Close it first", holders.Users[0].Name, holders.Users[0].PID)
	}
	_, err = d.command("stop")
	return err
}

func (d drive) verifyRuntime() error {
	rel, err := filepath.Rel(d.root, d.runtime)
	if err != nil || !filepath.IsAbs(d.root) || !filepath.IsAbs(d.runtime) || !filepath.IsLocal(rel) || filepath.Base(rel) != "svalbard-drive" ||
		!strings.HasPrefix(rel, filepath.Join(".svalbard", "runtime")+string(filepath.Separator)) {
		return fmt.Errorf("select a prepared Svalbard drive")
	}
	return nil
}
