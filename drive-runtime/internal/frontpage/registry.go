package frontpage

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

type serviceRecord struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Keys       []string  `json:"keys"`
	Endpoints  []string  `json:"endpoints,omitempty"`
	Resources  []string  `json:"resources,omitempty"`
	Started    time.Time `json:"started"`
	PID        int       `json:"pid,omitempty"`
	Executable string    `json:"executable,omitempty"`
	Events     []string  `json:"events,omitempty"`
	Inference  bool      `json:"inference,omitempty"`
}
type registrySnapshot struct {
	Jobs      map[string]state `json:"jobs"`
	Records   []*serviceRecord `json:"records"`
	Version   string           `json:"version"`
	Draining  bool             `json:"draining"`
	Terminals int              `json:"terminals"`
}

func appendUnique(dst []string, value string) []string {
	if value != "" && !slices.Contains(dst, value) {
		return append(dst, value)
	}
	return dst
}

// Called under d.mu. Only links published by a launcher can establish a
// shared listener. Process authority always remains with the owning job.
func (d *dashboard) registryLocked() registrySnapshot {
	snapshot := registrySnapshot{Jobs: map[string]state{}, Records: []*serviceRecord{}, Draining: d.draining, Terminals: d.terminals}
	groups := map[string]*serviceRecord{}
	for key, j := range d.jobs {
		snapshot.Jobs[key] = j.state
		spec := d.specs[key]
		links := j.Links
		if len(links) == 0 {
			links = []link{{}}
		}
		for _, link := range links {
			id := "job/" + key
			name := spec.name
			if link.URL != "" {
				address, err := url.Parse(link.URL)
				if err == nil && address.Host != "" {
					id = "listener/" + address.Host
				}
				if spec.name == "serve-all" {
					name = link.Label
				}
			}
			if strings.HasPrefix(name, "Wiki (") {
				name = "wiki"
			}
			if name == "Map" {
				name = "Browse drive files"
			}
			if spec.name == "browse" || name == "Kiwix" {
				name = "Kiwix"
			}
			record := groups[id]
			if record == nil {
				record = &serviceRecord{ID: id, Name: name, Status: j.Status, Started: j.started}
				groups[id] = record
				snapshot.Records = append(snapshot.Records, record)
			}
			if j.started.Before(record.Started) {
				record.Started = j.started
			}
			if j.Status == "running" {
				record.Status = "running"
			}
			record.Keys = appendUnique(record.Keys, key)
			record.Endpoints = appendUnique(record.Endpoints, link.URL)
			record.Inference = record.Inference || spec.name == "open-webui" || spec.name == "serve-all"
			for _, label := range d.resourceLabels[key] {
				record.Resources = appendUnique(record.Resources, label)
			}
			if len(d.resourceLabels[key]) == 0 {
				record.Resources = appendUnique(record.Resources, name)
			}
			for _, event := range j.events {
				record.Events = appendUnique(record.Events, event)
			}
			switch spec.name {
			case "maps", "documents", "workspace", "apps", "mcp-serve", "share":
				if j.Status == "running" {
					record.PID = os.Getpid()
					record.Executable = "svalbard-drive"
				}
			}
			if name == "Kiwix" && j.pid != 0 && j.Status == "running" {
				record.PID = j.pid
				record.Executable = "kiwix-serve"
			}
		}
	}
	for _, record := range snapshot.Records {
		slices.Sort(record.Keys)
		slices.Sort(record.Resources)
		slices.Sort(record.Endpoints)
		slices.Sort(record.Events)
	}
	slices.SortFunc(snapshot.Records, func(a, b *serviceRecord) int { return strings.Compare(a.ID, b.ID) })
	evidence, _ := json.Marshal(snapshot.Records)
	snapshot.Version = fmt.Sprintf("%x", sha256.Sum256(evidence))
	return snapshot
}
func (d *dashboard) registry(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	snapshot := d.registryLocked()
	d.mu.Unlock()
	writeJSON(w, snapshot)
}

func activeJob(status string) bool {
	return status == "running" || status == "starting" || status == "stopping"
}

func (d *dashboard) lifecycle(w http.ResponseWriter, r *http.Request) {
	var input struct{ Action, ID, Version string }
	if !readJSON(w, r, &input) {
		return
	}
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()
	d.mu.Lock()
	if input.Action == "resume" {
		d.draining = false
		d.mu.Unlock()
		writeJSON(w, map[string]string{"message": "New launches are enabled."})
		return
	}
	view := d.registryLocked()
	if view.Version != input.Version {
		d.mu.Unlock()
		http.Error(w, "Services changed. Review the current selection and try again.", 409)
		return
	}
	var keys []string
	switch input.Action {
	case "stop-all", "prepare":
		for key, j := range d.jobs {
			if activeJob(j.Status) {
				keys = append(keys, key)
			}
		}
		if input.Action == "prepare" {
			if d.terminals > 0 {
				d.mu.Unlock()
				http.Error(w, "Disconnect open terminals before preparing the drive.", 409)
				return
			}
			d.draining = true
		}
	case "stop", "restart":
		for _, record := range view.Records {
			if record.ID == input.ID {
				keys = record.Keys
				break
			}
		}
		if len(keys) == 0 || d.draining {
			d.mu.Unlock()
			http.Error(w, "Service is unavailable for this action.", 409)
			return
		}
	default:
		d.mu.Unlock()
		http.Error(w, "Unknown service action", 400)
		return
	}
	jobs := make(map[string]*job, len(keys))
	for _, key := range keys {
		j := d.jobs[key]
		jobs[key] = j
		if activeJob(j.Status) {
			j.Status = "stopping"
			j.cancel()
		}
	}
	d.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	for _, j := range jobs {
		select {
		case <-j.done:
		case <-ctx.Done():
			http.Error(w, "Stop is still pending. Check the register for remaining services.", 503)
			return
		}
	}
	if d.kiwix != nil {
		d.kiwix.StopIdle()
	}
	if input.Action == "restart" {
		var restarted []*job
		d.mu.Lock()
		for key, j := range jobs {
			if d.ctx.Err() != nil || d.jobs[key] != j {
				d.mu.Unlock()
				http.Error(w, "Service changed while restarting.", 409)
				return
			}
			restarted = append(restarted, d.launchLocked(key, d.specs[key], j.model))
		}
		d.mu.Unlock()
		for _, job := range restarted {
			select {
			case <-job.ready:
				d.mu.Lock()
				status := job.Status
				d.mu.Unlock()
				if status != "running" && status != "done" {
					http.Error(w, "Restart failed. Review the service register.", 503)
					return
				}
			case <-ctx.Done():
				http.Error(w, "Restart is still pending. The register will update when the service is ready.", 503)
				return
			}
		}
	}
	message := "Service register updated."
	if input.Action == "prepare" {
		// Wait for any settings writer and release the search database, model
		// and search-owned readers before inspecting the drive.
		d.settingsMu.Lock()
		d.settingsMu.Unlock()
		d.searchMu.Lock()
		if d.search != nil {
			d.search.Close()
			d.search = nil
		}
		d.searchMu.Unlock()
		message = removalStatus(ctx, d.root)
	}
	writeJSON(w, map[string]string{"message": message})
}

// An incomplete scan cannot certify safe removal. Only report matching PIDs,
// never names of the user's open files or arguments from other processes.
type driveProcess struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
}

func driveUsers(ctx context.Context, root string) ([]driveProcess, bool) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lsof", "-nP", "-F", "pc", "+D", filepath.Clean(root))
	out, err := cmd.Output()
	var users []driveProcess
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			if pid, e := strconv.Atoi(line[1:]); e == nil {
				users = append(users, driveProcess{PID: pid})
			}
		case 'c':
			if len(users) > 0 {
				users[len(users)-1].Name = filepath.Base(line[1:])
			}
		}
	}
	users = slices.DeleteFunc(users, func(p driveProcess) bool { return p.PID == os.Getpid() })
	complete := err == nil
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 && len(exit.Stderr) == 0 && ctx.Err() == nil {
		complete = true
	}
	return users, complete
}

func (d *dashboard) inspectDriveUsers(w http.ResponseWriter, r *http.Request) {
	users, complete := driveUsers(r.Context(), d.root)
	d.mu.Lock()
	reg := d.registryLocked()
	d.mu.Unlock()
	users = slices.DeleteFunc(users, func(p driveProcess) bool {
		return slices.ContainsFunc(reg.Records, func(record *serviceRecord) bool { return record.PID == p.PID && record.Status == "running" })
	})
	writeJSON(w, struct {
		Users    []driveProcess `json:"users"`
		Complete bool           `json:"complete"`
	}{users, complete})
}
func removalStatus(ctx context.Context, root string) string {
	suffix := " Stop ./run web, then use the operating system’s Eject command."
	users, complete := driveUsers(ctx, root)
	if len(users) > 0 {
		var ids []string
		for _, p := range users {
			ids = append(ids, strconv.Itoa(p.PID))
		}
		return "Removal blocked. Other processes hold files on the drive (PID " + strings.Join(ids, ", ") + "). Close them and check again." + suffix
	}
	if !complete {
		return "Managed services stopped. Remaining operating-system handles could not be verified." + suffix
	}
	return "Managed services stopped. No other open handles were reported for this user. The dashboard still holds the drive." + suffix
}
