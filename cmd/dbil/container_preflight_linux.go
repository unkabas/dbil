//go:build linux

package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/unkabas/dbil/internal/config"
)

const (
	containerRuntimeUID = 65532
	containerRuntimeGID = 0
)

func prepareContainerRuntime(cfg config.DBilConfig) error {
	if !shouldPrepareContainerRuntime() {
		return nil
	}
	if err := ensureContainerDataDir(cfg.DataDir); err != nil {
		return err
	}
	if err := dropContainerPrivileges(); err != nil {
		return err
	}
	if err := os.Chmod(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("container preflight: chmod data dir after privilege drop: %w", err)
	}
	slog.Info("container privileges dropped",
		"uid", containerRuntimeUID,
		"gid", containerRuntimeGID,
		"data_dir", cfg.DataDir)
	return nil
}

func shouldPrepareContainerRuntime() bool {
	if os.Geteuid() != 0 {
		return false
	}
	if disablesPrivilegeDrop(os.Getenv("DBIL_DROP_PRIVILEGES")) {
		return false
	}
	return runningInContainer()
}

func disablesPrivilegeDrop(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "no", "off":
		return true
	default:
		return false
	}
}

func runningInContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	body, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}
	s := string(body)
	return strings.Contains(s, "docker") ||
		strings.Contains(s, "kubepods") ||
		strings.Contains(s, "containerd")
}

func ensureContainerDataDir(dataDir string) error {
	if dataDir == "" {
		return fmt.Errorf("container preflight: data dir is empty")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("container preflight: mkdir data dir: %w", err)
	}
	ownedByRuntime, err := pathOwnedByUID(dataDir, containerRuntimeUID)
	if err != nil {
		return err
	}
	if ownedByRuntime {
		return nil
	}
	if err := chownTree(dataDir, containerRuntimeUID, containerRuntimeGID); err != nil {
		return fmt.Errorf("container preflight: data dir ownership: %w", err)
	}
	return nil
}

func chownTree(path string, uid, gid int) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := chownTree(filepath.Join(path, entry.Name()), uid, gid); err != nil {
				return err
			}
		}
	}
	if err := os.Lchown(path, uid, gid); err != nil {
		return fmt.Errorf("chown %s: %w", path, err)
	}
	return nil
}

func pathOwnedByUID(path string, uid int) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("container preflight: stat data dir: %w", err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("container preflight: stat data dir: unsupported stat type %T", info.Sys())
	}
	return int(st.Uid) == uid, nil
}

func dropContainerPrivileges() error {
	if err := syscall.Setgroups([]int{containerRuntimeGID}); err != nil {
		return fmt.Errorf("container preflight: setgroups: %w", err)
	}
	if err := syscall.Setgid(containerRuntimeGID); err != nil {
		return fmt.Errorf("container preflight: setgid: %w", err)
	}
	if err := syscall.Setuid(containerRuntimeUID); err != nil {
		return fmt.Errorf("container preflight: setuid: %w", err)
	}
	if os.Geteuid() != containerRuntimeUID {
		return fmt.Errorf("container preflight: effective uid is %d after drop, want %d", os.Geteuid(), containerRuntimeUID)
	}
	return nil
}
