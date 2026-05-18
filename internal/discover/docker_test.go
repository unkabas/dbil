package discover

import (
	"context"
	"errors"
	"testing"

	"github.com/unkabas/dbil/internal/dockerapi"
)

type fakeLister struct {
	list    []dockerapi.Summary
	inspect map[string]dockerapi.Inspect
	err     error
}

func (f *fakeLister) ContainerList(_ context.Context) ([]dockerapi.Summary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

func (f *fakeLister) ContainerInspect(_ context.Context, id string) (dockerapi.Inspect, error) {
	r, ok := f.inspect[id]
	if !ok {
		return dockerapi.Inspect{}, errors.New("not found")
	}
	return r, nil
}

func TestDockerScanner_HappyPath(t *testing.T) {
	labels := map[string]string{
		LabelEnable:      "true",
		LabelAlias:       "App DB",
		LabelTag:         "dev",
		LabelUsernameEnv: "POSTGRES_USER",
		LabelPasswordEnv: "POSTGRES_PASSWORD",
		LabelDatabaseEnv: "POSTGRES_DB",
	}
	c := dockerapi.Summary{
		ID:     "abc1234567890def",
		Names:  []string{"/postgres"},
		Labels: labels,
		NetworkSettings: &dockerapi.NetworkSettingsSummary{
			Networks: map[string]*dockerapi.EndpointSettings{"appnet": {IPAddress: "172.0.0.2"}},
		},
	}
	insp := dockerapi.Inspect{
		Config: &dockerapi.Config{
			Env: []string{"POSTGRES_USER=app", "POSTGRES_PASSWORD=s3cret", "POSTGRES_DB=appdb"},
		},
	}
	s := &DockerScanner{
		Client:  &fakeLister{list: []dockerapi.Summary{c}, inspect: map[string]dockerapi.Inspect{c.ID: insp}},
		Network: "appnet",
	}
	out, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("count: %d", len(out))
	}
	e := out[0]
	if e.Alias != "App DB" || e.Username != "app" || e.Password != "s3cret" || e.Database != "appdb" {
		t.Fatalf("entry: %+v", e)
	}
	if e.Host != "postgres" || e.Port != 5432 || e.Tag != "dev" {
		t.Fatalf("host/port/tag: %+v", e)
	}
	if e.Source != SourceDocker || e.Key != "abc1234567890def" {
		t.Fatalf("source/key: %+v", e)
	}
}

func TestDockerScanner_SkipsDisabled(t *testing.T) {
	c := dockerapi.Summary{ID: "x", Names: []string{"/p"}, Labels: map[string]string{}}
	s := &DockerScanner{Client: &fakeLister{list: []dockerapi.Summary{c}}}
	out, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("want 0, got %+v", out)
	}
}

func TestDockerScanner_SkipsWrongNetwork(t *testing.T) {
	c := dockerapi.Summary{
		ID:     "x",
		Names:  []string{"/p"},
		Labels: map[string]string{LabelEnable: "true"},
		NetworkSettings: &dockerapi.NetworkSettingsSummary{
			Networks: map[string]*dockerapi.EndpointSettings{"bridge": {}},
		},
	}
	s := &DockerScanner{
		Client:  &fakeLister{list: []dockerapi.Summary{c}},
		Network: "appnet",
	}
	out, _ := s.Scan(context.Background())
	if len(out) != 0 {
		t.Fatalf("expected skip, got %+v", out)
	}
}

func TestDockerScanner_SkipsMissingEnv(t *testing.T) {
	labels := map[string]string{
		LabelEnable:      "true",
		LabelUsernameEnv: "POSTGRES_USER",
		LabelDatabaseEnv: "POSTGRES_DB",
	}
	c := dockerapi.Summary{
		ID:     "x",
		Names:  []string{"/p"},
		Labels: labels,
	}
	insp := dockerapi.Inspect{
		Config: &dockerapi.Config{Env: []string{"POSTGRES_DB=appdb"}},
	}
	s := &DockerScanner{Client: &fakeLister{
		list:    []dockerapi.Summary{c},
		inspect: map[string]dockerapi.Inspect{c.ID: insp},
	}}
	out, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected skip on missing user env, got %+v", out)
	}
}

func TestDockerScanner_InvalidPortRejected(t *testing.T) {
	labels := map[string]string{
		LabelEnable:      "true",
		LabelPort:        "99999",
		LabelUsernameEnv: "U",
		LabelDatabaseEnv: "D",
	}
	c := dockerapi.Summary{ID: "x", Names: []string{"/p"}, Labels: labels}
	insp := dockerapi.Inspect{Config: &dockerapi.Config{Env: []string{"U=u", "D=d"}}}
	s := &DockerScanner{Client: &fakeLister{
		list:    []dockerapi.Summary{c},
		inspect: map[string]dockerapi.Inspect{c.ID: insp},
	}}
	out, _ := s.Scan(context.Background())
	if len(out) != 0 {
		t.Fatalf("expected port rejection, got %+v", out)
	}
}

func TestDockerScanner_PasswordOptional(t *testing.T) {
	labels := map[string]string{
		LabelEnable:      "true",
		LabelUsernameEnv: "U",
		LabelDatabaseEnv: "D",
	}
	c := dockerapi.Summary{ID: "x", Names: []string{"/p"}, Labels: labels}
	insp := dockerapi.Inspect{Config: &dockerapi.Config{Env: []string{"U=u", "D=d"}}}
	s := &DockerScanner{Client: &fakeLister{
		list:    []dockerapi.Summary{c},
		inspect: map[string]dockerapi.Inspect{c.ID: insp},
	}}
	out, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Password != "" {
		t.Fatalf("expected passwordless entry, got %+v", out)
	}
}
