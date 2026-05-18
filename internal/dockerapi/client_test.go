package dockerapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContainerList_ParsesShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/json" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
		  {
		    "Id":"abc1234",
		    "Names":["/postgres"],
		    "Labels":{"dbil.enable":"true"},
		    "NetworkSettings":{"Networks":{"appnet":{"IPAddress":"172.0.0.2"}}}
		  }
		]`))
	}))
	defer srv.Close()

	c := &Client{http: srv.Client(), base: srv.URL}
	list, err := c.ContainerList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "abc1234" {
		t.Fatalf("list: %+v", list)
	}
	if list[0].Labels["dbil.enable"] != "true" {
		t.Fatalf("labels: %+v", list[0].Labels)
	}
	ep := list[0].NetworkSettings.Networks["appnet"]
	if ep == nil || ep.IPAddress != "172.0.0.2" {
		t.Fatalf("endpoint: %+v", ep)
	}
}

func TestContainerInspect_ParsesEnvAndNetworks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/containers/abc/json") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
		  "Config":{"Env":["POSTGRES_USER=app","POSTGRES_PASSWORD=s","POSTGRES_DB=db"]},
		  "NetworkSettings":{"Networks":{"appnet":{"IPAddress":"172.0.0.2"}}}
		}`))
	}))
	defer srv.Close()

	c := &Client{http: srv.Client(), base: srv.URL}
	insp, err := c.ContainerInspect(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if insp.Config == nil || len(insp.Config.Env) != 3 {
		t.Fatalf("config: %+v", insp.Config)
	}
	if insp.NetworkSettings == nil || insp.NetworkSettings.Networks["appnet"].IPAddress != "172.0.0.2" {
		t.Fatalf("networks: %+v", insp.NetworkSettings)
	}
}

func TestContainerInspect_RejectsEmptyID(t *testing.T) {
	c := &Client{}
	if _, err := c.ContainerInspect(context.Background(), ""); err == nil {
		t.Fatal("expected empty-id error")
	}
}

func TestGet_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	c := &Client{http: srv.Client(), base: srv.URL}
	if _, err := c.ContainerList(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "403") {
		t.Fatalf("expected 403 surface, got %v", err)
	}
}
