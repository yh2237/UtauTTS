package updatelock

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const Suffix = ".update-lock.json"

const fallbackPrefix = "utautts-update-lock-"

type State struct {
	Version    string    `json:"version,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	UpdaterPID int       `json:"updater_pid"`
}

func Path(target string) (string, error) {
	absolute, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute) + Suffix, nil
}

func FallbackPath(target string) (string, error) {
	absolute, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	key := filepath.ToSlash(filepath.Clean(absolute))
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(os.TempDir(), fallbackPrefix+fmt.Sprintf("%x", hash)+".json"), nil
}

func Paths(target string) ([]string, error) {
	primary, err := Path(target)
	if err != nil {
		return nil, err
	}
	fallback, err := FallbackPath(target)
	if err != nil {
		return nil, err
	}
	return []string{primary, fallback}, nil
}

func marshalState(version string, pid int) ([]byte, error) {
	state := State{Version: version, StartedAt: time.Now().UTC(), UpdaterPID: pid}
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func writeFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write update lock %s: %w", path, err)
	}
	return nil
}

func WritePrimary(target, version string, pid int) error {
	path, err := Path(target)
	if err != nil {
		return err
	}
	data, err := marshalState(version, pid)
	if err != nil {
		return err
	}
	return writeFile(path, data)
}

func WriteFallback(target, version string, pid int) error {
	path, err := FallbackPath(target)
	if err != nil {
		return err
	}
	data, err := marshalState(version, pid)
	if err != nil {
		return err
	}
	return writeFile(path, data)
}

func Write(target, version string, pid int) error {
	paths, err := Paths(target)
	if err != nil {
		return err
	}
	data, err := marshalState(version, pid)
	if err != nil {
		return err
	}
	var failures []error
	for _, path := range paths {
		if err := writeFile(path, data); err == nil {
			return nil
		} else {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func PrimaryWritable(target string) bool {
	path, err := Path(target)
	if err != nil {
		return false
	}
	marker := filepath.Join(filepath.Dir(path),
		fmt.Sprintf(".utautts-update-permission-%d-%d", os.Getpid(), time.Now().UnixNano()))
	file, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false
	}
	if closeErr := file.Close(); closeErr != nil {
		_ = os.Remove(marker)
		return false
	}
	return os.Remove(marker) == nil
}

func Read(target string) (State, error) {
	paths, err := Paths(target)
	if err != nil {
		return State{}, err
	}
	var firstErr error
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("read update lock %s: %w", path, err)
			}
			continue
		}
		var state State
		if err := json.Unmarshal(data, &state); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("decode update lock %s: %w", path, err)
			}
			continue
		}
		return state, nil
	}
	if firstErr != nil {
		return State{}, firstErr
	}
	return State{}, os.ErrNotExist
}

func Remove(target string) error {
	paths, err := Paths(target)
	if err != nil {
		return err
	}
	var failures []error
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			failures = append(failures, fmt.Errorf("remove update lock %s: %w", path, err))
		}
	}
	return errors.Join(failures...)
}
