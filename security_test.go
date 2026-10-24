package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestIncludeShellSyntaxIsLiteral(t *testing.T) {
	for _, kind := range []string{"substitution", "backticks", "semicolon"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			marker := filepath.Join(home, "must-not-exist")
			patterns := map[string]string{
				"substitution": "$(touch " + marker + ")",
				"backticks":    "`touch " + marker + "`",
				"semicolon":    "settings; touch " + marker,
			}
			pattern := patterns[kind]
			root := filepath.Join(home, ".ssh", "config")
			writeConfig(t, root, "Include \""+pattern+"\"\n")
			// This is a literal filename, even though it contains shell syntax.
			writeConfig(t, filepath.Join(home, ".ssh", pattern), "Host literal-path\n")
			got, err := getSSHHostList(root, home)
			if err != nil || !reflect.DeepEqual(got, []string{"literal-path"}) {
				t.Fatalf("hosts=%v err=%v", got, err)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("Include executed a command")
			}
		})
	}
}

func TestDiscoveryDoesNotExecuteSSHDirectives(t *testing.T) {
	home := t.TempDir()
	marker := filepath.Join(home, "must-not-exist")
	root := filepath.Join(home, ".ssh", "config")
	writeConfig(t, root, "Match exec \"touch "+marker+"\"\nInclude included\nHost root\nProxyCommand touch "+marker+"\nLocalCommand touch "+marker+"\n")
	writeConfig(t, filepath.Join(home, ".ssh", "included"), "Host included\n")
	got, err := getSSHHostList(root, home)
	if err != nil || !reflect.DeepEqual(got, []string{"included", "root"}) {
		t.Fatalf("hosts=%v err=%v", got, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("SSH directive was executed during discovery")
	}
}

func TestIncludeRejectsHiddenCharacters(t *testing.T) {
	for name, char := range map[string]string{
		"NUL": "\x00", "escape": "\x1b", "carriage return": "\r",
		"quoted tab": "\t", "zero width space": "\u200b",
		"bidi override": "\u202e", "line separator": "\u2028", "invalid UTF-8": "\xff",
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, ".ssh", "config")
			writeConfig(t, root, "Include \"safe"+char+"name\"\n")
			if _, err := getSSHHostList(root, home); err == nil {
				t.Fatal("expected unsafe path to be rejected")
			}
		})
	}
}

func TestIncludeRejectsEmptyPaths(t *testing.T) {
	for _, value := range []string{"\"\"", " # no path", ""} {
		home := t.TempDir()
		root := filepath.Join(home, ".ssh", "config")
		writeConfig(t, root, "Include "+value+"\n")
		if _, err := getSSHHostList(root, home); err == nil {
			t.Fatalf("accepted empty Include %q", value)
		}
	}
}

func TestIncludedAliasesRejectShellAndHiddenCharacters(t *testing.T) {
	for _, value := range []string{
		"safe;touch marker", "$(touch marker)", "`touch marker`", "safe|sh", "safe&sh",
		"safe'quote", "-oProxyCommand=sh", "safe\x00", "safe\x1b", "safe\u200b", "safe\u202e",
	} {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, ".ssh", "config")
			writeConfig(t, root, "Include included\n")
			writeConfig(t, filepath.Join(home, ".ssh", "included"), "Host \""+value+"\"\nHost safe-alias\n")
			got, err := getSSHHostList(root, home)
			if err != nil || !reflect.DeepEqual(got, []string{"safe-alias"}) {
				t.Fatalf("hosts=%v err=%v", got, err)
			}
			if _, err := sshWorkflowArg("Default", value, "sshHost"); err == nil {
				t.Fatal("accepted unsafe alias in the command builder")
			}
		})
	}
}

func TestShellQuoteDoesNotExecutePayload(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	for _, value := range []string{
		"$(touch " + marker + ")", "`touch " + marker + "`",
		"'; touch " + marker + "; echo '", "safe; touch " + marker,
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		out, err := exec.CommandContext(ctx, "/bin/sh", "-c", "printf '%s' "+shellQuote(value)).Output()
		cancel()
		if err != nil || string(out) != value {
			t.Fatalf("shell quote changed payload %q: %q, %v", value, out, err)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("payload was executed")
		}
	}
}

func TestSSHCommandDispatchUsesOneLiteralHost(t *testing.T) {
	arg, err := sshWorkflowArg("Default", "safe-alias", "sshHost")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(arg, "\n") != 1 {
		t.Fatal("unexpected AppleScript argument separator")
	}
	command := strings.SplitN(arg, "\n", 2)[1]
	stub := "ssh() { printf '<%s>' \"$@\" >&2; }; clear() { :; }; "
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/bin/sh", "-c", stub+command).CombinedOutput()
	if err != nil || !strings.HasSuffix(string(out), "<--><safe-alias>") {
		t.Fatalf("unexpected dispatch: %q, %v", out, err)
	}
	if !strings.HasPrefix(string(out), "\x1b]1337;SetUserVar=sshHost=c2FmZS1hbGlhcw==\a") {
		t.Fatalf("iTerm badge changed: %q", out)
	}
}

func TestWorkflowArgumentsRejectInjection(t *testing.T) {
	for _, profile := range []string{"", "Default\ncommand", "Default\rcommand", "Default\u202e"} {
		if _, err := sshWorkflowArg(profile, "safe", "sshHost"); err == nil {
			t.Fatal("accepted invalid profile")
		}
	}
	for _, key := range []string{"", "1key", "key;touch marker", "key\n", "key`command`"} {
		if _, err := sshWorkflowArg("Default", "safe", key); err == nil {
			t.Fatal("accepted invalid badge variable")
		}
	}
}

func TestIncludeRejectsNonRegularFiles(t *testing.T) {
	for _, kind := range []string{"directory", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, ".ssh", "config")
			included := filepath.Join(home, ".ssh", "included")
			writeConfig(t, root, "Include included\n")
			if kind == "directory" {
				if err := os.Mkdir(included, 0700); err != nil {
					t.Fatal(err)
				}
			} else if err := syscall.Mkfifo(included, 0600); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { _, err := getSSHHostList(root, home); done <- err }()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("accepted non-regular config")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("config discovery blocked on a non-regular file")
			}
		})
	}
}

func TestIncludeLimits(t *testing.T) {
	t.Run("file count", func(t *testing.T) {
		home := t.TempDir()
		root := filepath.Join(home, ".ssh", "config")
		writeConfig(t, root, "Include parts/*\n")
		for i := 0; i < 1024; i++ {
			writeConfig(t, filepath.Join(home, ".ssh", "parts", fmt.Sprint(i)), "# config\n")
		}
		if _, err := getSSHHostList(root, home); err == nil {
			t.Fatal("expected file count limit")
		}
	})
	t.Run("total size", func(t *testing.T) {
		home := t.TempDir()
		root := filepath.Join(home, ".ssh", "config")
		writeConfig(t, root, "Include parts/*\n")
		for i := 0; i < 5; i++ {
			writeConfig(t, filepath.Join(home, ".ssh", "parts", fmt.Sprint(i)), strings.Repeat("#comment\n", 110000))
		}
		if _, err := getSSHHostList(root, home); err == nil {
			t.Fatal("expected aggregate size limit")
		}
	})
	t.Run("depth", func(t *testing.T) {
		home := t.TempDir()
		root := filepath.Join(home, ".ssh", "config")
		writeConfig(t, root, "Include depth-0\n")
		for i := 0; i < 35; i++ {
			writeConfig(t, filepath.Join(home, ".ssh", fmt.Sprintf("depth-%d", i)), fmt.Sprintf("Include depth-%d\n", i+1))
		}
		if _, err := getSSHHostList(root, home); err == nil {
			t.Fatal("expected depth limit")
		}
	})
	t.Run("file size", func(t *testing.T) {
		home := t.TempDir()
		root := filepath.Join(home, ".ssh", "config")
		writeConfig(t, root, "Include huge\n")
		writeConfig(t, filepath.Join(home, ".ssh", "huge"), strings.Repeat("#comment\n", 140000))
		if _, err := getSSHHostList(root, home); err == nil {
			t.Fatal("expected size limit")
		}
	})
}

func FuzzSSHConfigWords(f *testing.F) {
	for _, seed := range []string{"", "~/config", "\"folder with spaces\"", "$(command)", "`command`", "a\x00b", "a\u202eb", "\xff", "a\\"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		words, err := sshConfigWords(value)
		if err != nil {
			return
		}
		for _, word := range words {
			arg, err := sshWorkflowArg("Default", word, "sshHost")
			if err == nil && strings.Count(arg, "\n") != 1 {
				t.Fatal("argument separator injection")
			}
		}
	})
}
