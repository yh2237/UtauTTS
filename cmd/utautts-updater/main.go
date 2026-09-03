package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"utautts/internal/updatelock"
)

var logPath = filepath.Join(os.TempDir(), "utautts-updater.log")

func logf(format string, args ...any) {
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
	if file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		_, _ = file.WriteString(line)
		_ = file.Close()
	}
	fmt.Print(line)
}

func main() {
	target := flag.String("target", "", "package root directory to update")
	downloadURL := flag.String("url", "", "zip download URL")
	zipFlag := flag.String("zip", "", "local zip file to install; skips download when set")
	deleteZip := flag.Bool("delete-zip", false, "delete the local zip after staging (for application-owned temporary downloads)")
	pid := flag.Int("pid", 0, "PID of the running GUI to wait for before replacing files")
	version := flag.String("version", "", "incoming release tag (diagnostics)")
	lockToken := flag.String("lock-token", "", "internal: token of the pending update lock")
	preserveFlag := flag.String("preserve", "voice,Resamplers,Wavtools,config.ini", "comma-separated relative paths kept from the old install")
	elevated := flag.Bool("elevated", false, "internal: updater was relaunched with administrator privileges")
	flag.Parse()

	if *target == "" || (*downloadURL == "" && *zipFlag == "") {
		logf("usage: utautts-updater -target <dir> (-url <zip-url> | -zip <file>) [-pid <pid>] [-version <tag>]")
		os.Exit(2)
	}
	var preserve []string
	for _, part := range strings.Split(*preserveFlag, ",") {
		if part = strings.TrimSpace(part); part != "" {
			preserve = append(preserve, part)
		}
	}

	ok := true
	lockOwned := false
	claimedToken := strings.TrimSpace(*lockToken)
	var lockErr error
	if claimedToken == "" {
		claimedToken, lockErr = updatelock.Acquire(*target, *version, os.Getpid())
	} else {
		lockErr = updatelock.WriteWithToken(*target, *version, os.Getpid(), claimedToken)
	}
	if lockErr != nil {
		ok = false
		logf("update lock failed: %v", lockErr)
	} else {
		lockOwned = true
		if !*elevated {
			args := append([]string{}, os.Args[1:]...)
			args = append(args, "-elevated")
			if strings.TrimSpace(*lockToken) == "" {
				args = append(args, "-lock-token", claimedToken)
			}
			relaunched, err := relaunchElevatedIfNeeded(*target, args)
			if err != nil {
				ok = false
				logf("administrator elevation failed: %v", err)
			} else if relaunched {
				return
			}
		}
	}
	if ok {
		if err := run(*target, *downloadURL, *zipFlag, *pid, *version, preserve, *deleteZip); err != nil {
			ok = false
			logf("update failed: %v", err)
		}
	}
	if !ok && lockOwned {
		_ = updatelock.Remove(*target)
	}
	if !lockOwned {
		os.Exit(1)
	}
	launchApp(*target)
	if err := updatelock.Remove(*target); err != nil {
		logf("removing update lock failed: %v", err)
	}
	if !ok {
		os.Exit(1)
	}
}

func detachFromTarget(target string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cwd = filepath.Clean(cwd)
	target = filepath.Clean(target)
	if cwd != target && !strings.HasPrefix(cwd, target+string(os.PathSeparator)) {
		return nil
	}
	if err := os.Chdir(filepath.Dir(target)); err != nil {
		return fmt.Errorf("cannot detach working directory from target: %w", err)
	}
	logf("working directory moved to %s", filepath.Dir(target))
	return nil
}

func run(target, url, zipPath string, pid int, version string, preserve []string, deleteLocalZip bool) error {
	logf("utautts-updater start: target=%s version=%s", target, version)
	absolute, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	target = absolute
	if cwd, err := os.Getwd(); err == nil {
		logf("working directory: %s", cwd)
	}
	if err := detachFromTarget(target); err != nil {
		return err
	}

	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		return fmt.Errorf("target directory is not a directory: %s", target)
	}

	stage := target + ".stage"
	old := target + ".old"

	downloaded := zipPath == ""
	if downloaded {
		zipPath = filepath.Join(os.TempDir(), "utautts-update-"+sanitizeToken(version)+".zip")
		if err := download(url, zipPath); err != nil {
			return err
		}
	} else {
		logf("using local archive: %s", zipPath)
		if info, err := os.Stat(zipPath); err != nil || info.IsDir() {
			return fmt.Errorf("local archive is not a file: %s", zipPath)
		}
	}

	if err := os.RemoveAll(stage); err != nil {
		return err
	}
	if err := extractZip(zipPath, stage); err != nil {
		return err
	}
	if downloaded || deleteLocalZip {
		if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
			logf("removing temporary archive failed: %v", err)
		}
	}
	if err := normalizeStage(stage); err != nil {
		return err
	}

	for _, rel := range preserve {
		if err := preservePath(target, stage, rel); err != nil {
			_ = os.RemoveAll(stage)
			return fmt.Errorf("preserve %s: %w", rel, err)
		}
	}
	if !waitForExit(pid, 5*time.Minute) {
		_ = os.RemoveAll(stage)
		return fmt.Errorf("parent process %d did not exit within timeout", pid)
	}

	if err := os.RemoveAll(old); err != nil {
		return err
	}
	if err := retry(20, 500*time.Millisecond, func() error {
		return os.Rename(target, old)
	}); err != nil {
		return fmt.Errorf("move current install aside: %w", err)
	}
	if err := retry(20, 500*time.Millisecond, func() error {
		return os.Rename(stage, target)
	}); err != nil {
		_ = os.Rename(old, target)
		return fmt.Errorf("move new install into place: %w", err)
	}
	if err := retry(20, 500*time.Millisecond, func() error {
		return os.RemoveAll(old)
	}); err != nil {
		logf("removing old install backup failed (left at %s): %v", old, err)
	}
	logf("update applied to %s", target)
	return nil
}

func retry(attempts int, delay time.Duration, fn func() error) error {
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if attempt+1 < attempts {
			time.Sleep(delay)
		}
	}
	return err
}

func waitForExit(pid int, timeout time.Duration) bool {
	if pid <= 0 {
		return true
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func download(url, dest string) error {
	logf("downloading %s", url)
	client := &http.Client{Timeout: 20 * time.Minute}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "UtauTTS-updater")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", response.Status)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	written, err := io.Copy(out, response.Body)
	if err != nil {
		return err
	}
	logf("downloaded %d bytes to %s", written, dest)
	return nil
}

func extractZip(zipPath, dest string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()
	dest = filepath.Clean(dest)
	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		path := filepath.Join(dest, name)
		if path != dest && !strings.HasPrefix(path, dest+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry escapes destination: %s", name)
		}
		if file.FileInfo().IsDir() || strings.HasSuffix(name, "/") {
			if err := ensureDir(path); err != nil {
				return err
			}
			continue
		}
		if err := ensureDir(filepath.Dir(path)); err != nil {
			return err
		}
		if info, err := os.Lstat(path); err == nil && info.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		mode := file.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		_ = in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if err := os.Chmod(path, mode); err != nil {
			return err
		}
	}
	return nil
}

func ensureDir(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if info.IsDir() {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return os.MkdirAll(path, 0o755)
}

func normalizeStage(stage string) error {
	if packageLooksValid(stage) {
		return nil
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return fmt.Errorf("stage does not look like a UtauTTS package: %s", stage)
	}
	inner := filepath.Join(stage, entries[0].Name())
	if !packageLooksValid(inner) {
		return fmt.Errorf("stage does not look like a UtauTTS package: %s", stage)
	}
	innerEntries, err := os.ReadDir(inner)
	if err != nil {
		return err
	}
	for _, entry := range innerEntries {
		if err := os.Rename(filepath.Join(inner, entry.Name()), filepath.Join(stage, entry.Name())); err != nil {
			return err
		}
	}
	return os.RemoveAll(inner)
}

func packageLooksValid(dir string) bool {
	for _, executable := range []string{
		filepath.Join("app", "utautts-gui.exe"),
		"utautts.exe",
		"utautts",
	} {
		if info, err := os.Stat(filepath.Join(dir, executable)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func preservePath(target, stage, rel string) error {
	clean, err := safePreservePath(rel)
	if err != nil {
		return err
	}
	source := filepath.Join(target, clean)
	destination := filepath.Join(stage, clean)
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			logf("preserve path is absent; skipping: %s", source)
			return nil
		}
		return fmt.Errorf("inspect source %s: %w", source, err)
	}
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	return copyTree(source, destination)
}

func safePreservePath(value string) (string, error) {
	normalized := strings.ReplaceAll(value, `\`, "/")
	hasWindowsVolume := len(normalized) >= 2 &&
		((normalized[0] >= 'a' && normalized[0] <= 'z') || (normalized[0] >= 'A' && normalized[0] <= 'Z')) &&
		normalized[1] == ':'
	if value == "" || strings.ContainsRune(value, 0) || path.IsAbs(normalized) ||
		filepath.IsAbs(value) || filepath.VolumeName(value) != "" || hasWindowsVolume {
		return "", fmt.Errorf("preserve path must be a relative path: %q", value)
	}
	clean := path.Clean(normalized)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("preserve path escapes the package root: %q", value)
	}
	return filepath.FromSlash(clean), nil
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		mode := fs.FileMode(0o644)
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
		return copyFile(path, target, mode)
	})
}

func copyFile(source, destination string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return os.Chmod(destination, mode)
}

func sanitizeToken(value string) string {
	if value == "" {
		return "latest"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, value)
}

func launchApp(target string) {
	var launcher string
	for _, relative := range []string{"utautts.exe", filepath.Join("app", "utautts-gui.exe"), "utautts"} {
		candidate := filepath.Join(target, relative)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			launcher = candidate
			break
		}
	}
	if launcher == "" {
		logf("cannot relaunch: no GUI executable found under %s", target)
		return
	}
	command := exec.Command(launcher)
	command.Dir = target
	command.Env = append(os.Environ(), "UTAUTTS_UPDATE_RELAUNCH=1")
	if err := command.Start(); err != nil {
		logf("relaunch failed: %v", err)
		return
	}
	logf("relaunched %s", launcher)
}
