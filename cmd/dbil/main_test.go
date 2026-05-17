package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCmd_Help(t *testing.T) {
	cmd := versionCmd()
	cmd.SetArgs([]string{})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "dbil") {
		t.Fatalf("expected 'dbil' in output, got %q", buf.String())
	}
}

func TestInitCmd_HelpDoesNotFail(t *testing.T) {
	cmd := initCmd()
	cmd.SetArgs([]string{"--help"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Initialize") {
		t.Fatalf("help output missing Init description: %q", buf.String())
	}
}
