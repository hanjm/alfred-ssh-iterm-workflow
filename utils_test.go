package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSSHHostListIncludes(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".ssh", "config")
	writeConfig(t, root, "Host root shared\nInclude parts/*.conf missing.conf ~/extra.conf\nHost tail\n")
	writeConfig(t, filepath.Join(home, ".ssh", "parts", "20-second.conf"), "Host second shared\n")
	writeConfig(t, filepath.Join(home, ".ssh", "parts", "10-first.conf"), "Host first\nInclude nested.conf\n")
	writeConfig(t, filepath.Join(home, ".ssh", "nested.conf"), "Host nested\n")
	writeConfig(t, filepath.Join(home, "extra.conf"), "Host extra\n")
	got, err := getSSHHostList(root, home)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"root", "shared", "first", "nested", "second", "extra", "tail"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hosts = %v, want %v", got, want)
	}
}

func TestSSHHostListSyntax(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".ssh", "config")
	writeConfig(t, root, "# Host commented\r\nHOST = one two\r\nHostname wrong\r\nHostKeyAlias wrong\r\nHost * *.example ?name !excluded\r\nHost one three # ignored\r\nInclude \"~/folder with spaces/hosts.conf\"\r\n")
	writeConfig(t, filepath.Join(home, "folder with spaces", "hosts.conf"), "Host quoted-path\n")
	got, err := getSSHHostList(root, home)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"one", "two", "three", "quoted-path"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hosts = %v, want %v", got, want)
	}
}

func TestSSHHostListAbsoluteAndRepeatedIncludes(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".ssh", "config")
	other := filepath.Join(home, "other.conf")
	writeConfig(t, root, "Include "+other+" "+other+"\nHost after\n")
	writeConfig(t, other, "Host included\nInclude config\n")
	got, err := getSSHHostList(root, home)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"included", "after"}) {
		t.Fatalf("unexpected hosts: %v", got)
	}
}

func TestSSHHostListSymlinkCycle(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".ssh", "config")
	writeConfig(t, root, "Host first\nInclude linked\nHost last\n")
	if err := os.Symlink(root, filepath.Join(home, ".ssh", "linked")); err != nil {
		t.Fatal(err)
	}
	got, err := getSSHHostList(root, home)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"first", "last"}) {
		t.Fatalf("unexpected hosts: %v", got)
	}
}

func TestSSHHostListErrors(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"quoted path", "Include \"unfinished\n"},
		{"bad glob", "Include [broken\n"},
		{"scanner error", "Host " + strings.Repeat("a", 70*1024)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, ".ssh", "config")
			writeConfig(t, root, tc.content)
			if _, err := getSSHHostList(root, home); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	t.Run("missing root", func(t *testing.T) {
		home := t.TempDir()
		if _, err := getSSHHostList(filepath.Join(home, ".ssh", "config"), home); err == nil {
			t.Fatal("expected missing root error")
		}
	})
}
