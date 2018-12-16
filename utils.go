package main

import (
	"bufio"
	"github.com/hanjm/errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// GetSSHHostList 从 ssh config文件中读取到所有的Host列表
func GetSSHHostList() (hosts []string, err error) {
	filePath, err := GetSSHConfigFilePath()
	if err != nil {
		return hosts, errors.Errorf(err, "")
	}
	fp, err := os.Open(filePath)
	if err != nil {
		return hosts, errors.Errorf(err, "failed to open file:%s", filePath)
	}
	defer fp.Close()
	sc := bufio.NewScanner(fp)
	sc.Split(bufio.ScanLines)
	const hostPrefix = "host"
	const hostPrefixLen = len(hostPrefix)
	const hostNamePrefix = "hostname"
	const hostNamePrefixLen = len(hostNamePrefix)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		lineLen := len(line)
		// is host prefix
		if lineLen > hostPrefixLen && strings.ToLower(line[:hostPrefixLen]) == hostPrefix {
			// not hostname prefix
			if lineLen > hostNamePrefixLen && strings.ToLower(line[:hostNamePrefixLen]) == hostNamePrefix {
				continue
			}
			host := strings.TrimSpace(line[hostPrefixLen:])
			if host != "" && host != "*" {
				hosts = append(hosts, host)
			}
		}
	}
	return hosts, nil
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
	// shell
	cmd := exec.Command("sh", "-c", "eval echo ~$USER")
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Errorf(nil, "failed to get home from shell, os:%s", runtime.GOOS)
	}
	if v := strings.TrimSpace(string(output)); v != "" {
		return v, nil
	}
	return "", errors.Errorf(nil, "failed to get home, os:%s", runtime.GOOS)
}
