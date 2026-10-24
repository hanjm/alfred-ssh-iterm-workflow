package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"github.com/hanjm/errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// GetSSHHostList 从 ssh config文件中读取到所有的Host列表
func GetSSHHostList() (hosts []string, err error) {
	filePath, err := GetSSHConfigFilePath()
	if err != nil {
		return hosts, errors.Errorf(err, "")
	}
	return getSSHHostList(filePath, filepath.Dir(filepath.Dir(filePath)))
}

// getSSHHostList discovers aliases, not effective per-host connection settings.
// OpenSSH still evaluates Host/Match conditions when a selected alias is used.
func getSSHHostList(filePath, homeDir string) ([]string, error) {
	r := sshHostReader{
		homeDir: homeDir,
		sshDir:  filepath.Join(homeDir, ".ssh"),
		files:   make(map[string]bool),
		hosts:   make(map[string]bool),
	}
	if err := r.read(filePath, 0); err != nil {
		return nil, err
	}
	return r.result, nil
}

type sshHostReader struct {
	homeDir string
	sshDir  string
	files   map[string]bool
	hosts   map[string]bool
	result  []string
	bytes   int64
}

func (r *sshHostReader) read(filePath string, depth int) error {
	if err := validateConfigText(filePath); err != nil {
		return fmt.Errorf("invalid SSH config path: %v", err)
	}
	canonical, err := filepath.EvalSymlinks(filePath)
	if err != nil {
		return errors.Errorf(err, "failed to resolve file:%s", filePath)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return err
	}
	if r.files[canonical] {
		return nil
	}
	if depth > 32 {
		return fmt.Errorf("SSH Include nesting exceeds 32 levels: %s", filePath)
	}
	if len(r.files) >= 1024 {
		return fmt.Errorf("SSH Include file count exceeds 1024")
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("SSH config is not a regular file: %s", filePath)
	}
	if info.Size() > 1024*1024 || r.bytes+info.Size() > 4*1024*1024 {
		return fmt.Errorf("SSH config size limit exceeded: %s", filePath)
	}
	r.bytes += info.Size()
	r.files[canonical] = true
	fp, err := os.Open(filePath)
	if err != nil {
		return errors.Errorf(err, "failed to open file:%s", filePath)
	}
	defer fp.Close()
	sc := bufio.NewScanner(fp)
	lineNumber := 0
	for sc.Scan() {
		lineNumber++
		if !utf8.ValidString(sc.Text()) {
			return fmt.Errorf("%s:%d: invalid UTF-8", filePath, lineNumber)
		}
		line := strings.TrimSpace(sc.Text())
		end := strings.IndexAny(line, " \t=")
		if end < 0 {
			if strings.EqualFold(line, "include") {
				return fmt.Errorf("%s:%d: missing Include path", filePath, lineNumber)
			}
			continue
		}
		directive := strings.ToLower(line[:end])
		if directive != "host" && directive != "include" {
			continue
		}
		values, err := sshConfigWords(strings.TrimLeft(line[end:], " \t="))
		if err != nil {
			return fmt.Errorf("%s:%d: %v", filePath, lineNumber, err)
		}
		if directive == "include" && len(values) == 0 {
			return fmt.Errorf("%s:%d: missing Include path", filePath, lineNumber)
		}
		for _, value := range values {
			if directive == "host" {
				if !validHostAlias(value) || r.hosts[value] {
					continue
				}
				r.hosts[value] = true
				r.result = append(r.result, value)
			} else if err := r.include(value, depth+1); err != nil {
				return fmt.Errorf("%s:%d: %v", filePath, lineNumber, err)
			}
		}
	}
	return sc.Err()
}

func (r *sshHostReader) include(pattern string, depth int) error {
	if pattern == "" {
		return fmt.Errorf("empty Include path")
	}
	if err := validateConfigText(pattern); err != nil {
		return fmt.Errorf("invalid Include path: %v", err)
	}
	if pattern == "~" {
		pattern = r.homeDir
	} else if strings.HasPrefix(pattern, "~/") {
		pattern = filepath.Join(r.homeDir, pattern[2:])
	} else if strings.HasPrefix(pattern, "~") {
		name, rest := pattern[1:], ""
		if slash := strings.IndexByte(name, '/'); slash >= 0 {
			name, rest = name[:slash], name[slash+1:]
		}
		account, err := user.Lookup(name)
		if err != nil {
			return errors.Errorf(err, "failed to expand Include user:%s", name)
		}
		pattern = filepath.Join(account.HomeDir, rest)
	}
	// Relative paths in a user's Include directive are relative to ~/.ssh,
	// including when the directive occurs in a file in another directory.
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(r.sshDir, pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return errors.Errorf(err, "invalid Include pattern:%s", pattern)
	}
	for _, match := range matches {
		if err := r.read(match, depth); err != nil {
			return err
		}
	}
	return nil
}

// sshConfigWords keeps quoted paths together and ignores unquoted comments.
func sshConfigWords(value string) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote rune
	escaped, started := false, false
	flush := func() {
		if started {
			words = append(words, word.String())
			word.Reset()
			started = false
		}
	}
	for _, char := range value {
		switch {
		case escaped:
			word.WriteRune(char)
			escaped = false
		case char == '\\' && quote != '\'':
			escaped, started = true, true
		case quote != 0:
			if char == quote {
				quote = 0
			} else {
				word.WriteRune(char)
			}
		case char == '"' || char == '\'':
			quote, started = char, true
		case char == '#':
			flush()
			return words, nil
		case char == ' ' || char == '\t':
			flush()
		default:
			word.WriteRune(char)
			started = true
		}
	}
	if quote != 0 || escaped {
		return nil, fmt.Errorf("unterminated quote or escape")
	}
	flush()
	return words, nil
}

// GetSSHConfigFile 得到ssh配置文件的路径
func GetSSHConfigFilePath() (filePath string, err error) {
	homeDir, err := GetHomeDir()
	if err != nil {
		return "", errors.Errorf(err, "")
	}
	filePath = filepath.Join(homeDir, ".ssh", "config")
	return filePath, nil
}

// GetHomeDir 得到用户的家目录
func GetHomeDir() (homeDir string, err error) {
	u, err := user.Current()
	if nil == err {
		return u.HomeDir, nil
	}
	// 环境变量
	if v := os.Getenv("HOME"); v != "" {
		return v, nil
	}
	return "", errors.Errorf(err, "failed to get home directory")
}

// Reject hidden control/format characters rather than passing them to paths,
// terminal escape sequences or the newline-delimited AppleScript argument.
func validateConfigText(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("invalid UTF-8")
	}
	for _, char := range value {
		if unicode.IsControl(char) || unicode.In(char, unicode.Cf, unicode.Zl, unicode.Zp) {
			return fmt.Errorf("control or invisible formatting character U+%04X", char)
		}
	}
	return nil
}

func validHostAlias(value string) bool {
	if value == "" || strings.HasPrefix(value, "-") || validateConfigText(value) != nil {
		return false
	}
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && !strings.ContainsRune("._-:%", char) {
			return false
		}
	}
	return true
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func sshWorkflowArg(profile, host, badgeKey string) (string, error) {
	if profile == "" || validateConfigText(profile) != nil || !validHostAlias(host) {
		return "", fmt.Errorf("invalid iTerm profile or SSH alias")
	}
	if badgeKey == "" {
		return "", fmt.Errorf("empty iTerm variable name")
	}
	for i, char := range badgeKey {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			char == '_' || (i > 0 && char >= '0' && char <= '9')) {
			return "", fmt.Errorf("invalid iTerm variable name")
		}
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(host))
	command := fmt.Sprintf(`printf '\033]1337;SetUserVar=%%s=%%s\007' %s %s; clear; ssh -- %s`,
		shellQuote(badgeKey), shellQuote(encoded), shellQuote(host))
	return profile + "\n" + command, nil
}
