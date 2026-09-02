package updatelock

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	Token      string    `json:"token,omitempty"`
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

func marshalState(version string, pid int, token string) ([]byte, error) {
	state := State{Version: version, StartedAt: time.Now().UTC(), UpdaterPID: pid, Token: token}
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
	data, err := marshalState(version, pid, "")
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
	data, err := marshalState(version, pid, "")
	if err != nil {
		return err
	}
	return writeFile(path, data)
}

func Write(target, version string, pid int) error {
	_, err := Acquire(target, version, pid)
	return err
}

func Acquire(target, version string, pid int) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate update lock token: %w", err)
	}
	token := hex.EncodeToString(bytes)
	if err := writeExclusive(target, version, pid, token); err != nil {
		return "", err
	}
	return token, nil
}

func WriteWithToken(target, version string, pid int, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return Write(target, version, pid)
	}
	paths, err := Paths(target)
	if err != nil {
		return err
	}
	data, err := marshalState(version, pid, token)
	if err != nil {
		return err
	}
	for _, path := range paths {
		state, readErr := readPath(path)
		if readErr != nil || state.Token != token {
			continue
		}
		return writeFile(path, data)
	}
	return fmt.Errorf("pending update lock token was not found")
}

func writeExclusive(target, version string, pid int, token string) error {
	paths, err := Paths(target)
	if err != nil {
		return err
	}
	data, err := marshalState(version, pid, token)
	if err != nil {
		return err
	}
	var failures []error
	for _, path := range paths {
		if _, statErr := os.Lstat(path); statErr == nil {
			return fmt.Errorf("update is already in progress: %s", path)
		} else if !os.IsNotExist(statErr) {
			// 検査できない場合はfallbackを試し、全て失敗したら原因を返す。
			failures = append(failures, fmt.Errorf("inspect update lock %s: %w", path, statErr))
		}
	}
	for _, path := range paths {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			if _, writeErr := file.Write(data); writeErr != nil {
				_ = file.Close()
				_ = os.Remove(path)
				return fmt.Errorf("write update lock %s: %w", path, writeErr)
			}
			if closeErr := file.Close(); closeErr != nil {
				_ = os.Remove(path)
				return fmt.Errorf("write update lock %s: %w", path, closeErr)
			}
			return nil
		}
		if os.IsExist(err) {
			return fmt.Errorf("update is already in progress: %s", path)
		}
		failures = append(failures, fmt.Errorf("write update lock %s: %w", path, err))
	}
	return errors.Join(failures...)
}

func readPath(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func ReadPath(path string) (State, error) {
	return readPath(path)
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
	var found State
	foundPath := ""
	for _, path := range paths {
		state, err := readPath(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("read update lock %s: %w", path, err)
			}
			continue
		}
		if foundPath == "" || state.StartedAt.After(found.StartedAt) {
			found = state
			foundPath = path
		}
	}
	if foundPath != "" {
		return found, nil
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
