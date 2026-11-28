package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceMenuListsLocalServicesOnce(t *testing.T) {
	s := serviceEntry{Label: "Archive", Service: "one", Icon: "browse"}
	var inv inventory
	inv.Groups = append(inv.Groups, serviceGroup{Items: []serviceEntry{
		s, s, {Label: "Network share", Service: "share", Icon: "share"},
		{Label: "Missing PDF", Service: "missing", Icon: "documents", Missing: true},
	}})
	services := inv.localServices()
	if len(services) != 1 || services[0] != s {
		t.Fatalf("local services: %+v", services)
	}
}

func TestDashboardClientRejectsRedirectsAndSendsMutationHeaders(t *testing.T) {
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("dashboard redirect escaped loopback instance")
	}))
	defer second.Close()
	var origin string
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/registry" {
			http.Redirect(w, r, second.URL, 307)
			return
		}
		if r.Method != "POST" || r.Header.Get("Origin") != origin || r.Header.Get("X-Svalbard-Request") != "1" {
			t.Errorf("wrong dashboard mutation: %v", r)
		}
		fmt.Fprint(w, "{}")
	}))
	defer first.Close()
	origin = first.URL
	if _, err := loopbackOrigin(origin + "/"); err != nil {
		t.Fatal(err)
	}
	for _, addr := range []string{"https://127.0.0.1:44/", "http://example.org:44/", origin + "/api"} {
		if _, err := loopbackOrigin(addr); err == nil {
			t.Errorf("accepted %s", addr)
		}
	}
	if err := dashboardRequest(origin, "/api/registry", nil, &registry{}); err == nil || !strings.Contains(err.Error(), "307") {
		t.Fatalf("registry redirect result: %v", err)
	}
	if err := dashboardRequest(origin, "/api/services/one", struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestActionsDoNotWaitForStatusPolls(t *testing.T) {
	var action menuAction
	if action.message() != "" || !action.begin("Opening frontpage…") {
		t.Fatal("a menu click should start immediately")
	}
	if action.begin("Stopping dashboard…") || action.message() != "Opening frontpage…" {
		t.Fatal("a second command should wait without clearing the current status")
	}
	action.finish(fmt.Errorf("Browser did not open"))
	if action.message() != "Browser did not open" {
		t.Fatal("a status refresh would hide the error")
	}
	if !action.begin("Stopping dashboard…") || action.message() != "Stopping dashboard…" {
		t.Fatal("retry could not replace the error")
	}
	action.finish(nil)
	if action.message() != "" {
		t.Fatal("completed action still showing")
	}
}
