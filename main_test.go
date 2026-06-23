package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// setupMocks points ipconfigURL and pushoverURL at throwaway test servers.
// Returns a restore function that must be deferred.
func setupMocks(t *testing.T, ip string, pushoverStatus int) func() {
	t.Helper()
	ipSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(ip + "\n"))
	}))
	poSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(pushoverStatus)
	}))
	origIP, origPO := ipconfigURL, pushoverURL
	ipconfigURL = ipSrv.URL
	pushoverURL = poSrv.URL
	return func() {
		ipconfigURL = origIP
		pushoverURL = origPO
		ipSrv.Close()
		poSrv.Close()
	}
}

// interceptExit replaces osExit with a recorder for the duration of the test.
// exitCode() returns the code passed to osExit, or -1 if it was never called.
func interceptExit(t *testing.T) (restore func(), exitCode func() int) {
	t.Helper()
	code := -1
	osExit = func(c int) { code = c }
	return func() { osExit = os.Exit }, func() int { return code }
}

// --- runOnce tests ---

func TestRunOnceFirstRun(t *testing.T) {
	defer setupMocks(t, "1.2.3.4", http.StatusOK)()
	restoreExit, exitCode := interceptExit(t)
	defer restoreExit()

	stateFile := filepath.Join(t.TempDir(), "ip")
	if err := runOnce("tok", "usr", "title", stateFile); err != nil {
		t.Fatal(err)
	}
	if exitCode() != -1 {
		t.Errorf("unexpected exit code %d on first run", exitCode())
	}
	if ip, err := readIP(stateFile); err != nil || ip != "1.2.3.4" {
		t.Errorf("state file: got %q %v, want 1.2.3.4", ip, err)
	}
}

func TestRunOnceIPUnchanged(t *testing.T) {
	defer setupMocks(t, "1.2.3.4", http.StatusOK)()
	restoreExit, exitCode := interceptExit(t)
	defer restoreExit()

	stateFile := filepath.Join(t.TempDir(), "ip")
	writeIP(stateFile, "1.2.3.4")

	if err := runOnce("tok", "usr", "title", stateFile); err != nil {
		t.Fatal(err)
	}
	if exitCode() != -1 {
		t.Errorf("unexpected exit code %d when IP unchanged", exitCode())
	}
}

func TestRunOnceIPChanged(t *testing.T) {
	defer setupMocks(t, "5.6.7.8", http.StatusOK)()
	restoreExit, exitCode := interceptExit(t)
	defer restoreExit()

	stateFile := filepath.Join(t.TempDir(), "ip")
	writeIP(stateFile, "1.2.3.4")

	runOnce("tok", "usr", "title", stateFile) //nolint:errcheck — osExit is mocked so nil is returned

	if exitCode() != 1 {
		t.Errorf("expected exit 1 on IP change, got %d", exitCode())
	}
	if ip, _ := readIP(stateFile); ip != "5.6.7.8" {
		t.Errorf("state file not updated: got %q, want 5.6.7.8", ip)
	}
}

func TestRunOnceIPChangedNotifyFailure(t *testing.T) {
	defer setupMocks(t, "5.6.7.8", http.StatusInternalServerError)()
	restoreExit, exitCode := interceptExit(t)
	defer restoreExit()

	stateFile := filepath.Join(t.TempDir(), "ip")
	writeIP(stateFile, "1.2.3.4")

	runOnce("tok", "usr", "title", stateFile) //nolint:errcheck

	// Notification failure is a warning; state should still update and exit 1.
	if exitCode() != 1 {
		t.Errorf("expected exit 1 even with notify failure, got %d", exitCode())
	}
	if ip, _ := readIP(stateFile); ip != "5.6.7.8" {
		t.Errorf("state file not updated: got %q, want 5.6.7.8", ip)
	}
}

func TestRunOnceFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()
	orig := ipconfigURL
	ipconfigURL = srv.URL
	defer func() { ipconfigURL = orig }()

	stateFile := filepath.Join(t.TempDir(), "ip")
	if err := runOnce("tok", "usr", "title", stateFile); err == nil {
		t.Fatal("expected error from closed server, got nil")
	}
}

// --- runDaemon tests ---

func TestRunDaemonChecksImmediately(t *testing.T) {
	defer setupMocks(t, "1.2.3.4", http.StatusOK)()

	stateFile := filepath.Join(t.TempDir(), "ip")

	done := make(chan error, 1)
	go func() {
		done <- runDaemon("tok", "usr", "title", stateFile, 10*time.Millisecond)
	}()

	// Allow at least one check cycle.
	time.Sleep(50 * time.Millisecond)
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runDaemon returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runDaemon did not stop after SIGTERM")
	}

	// State file should have been written by the first check.
	if ip, err := readIP(stateFile); err != nil || ip != "1.2.3.4" {
		t.Errorf("state file after first check: got %q %v, want 1.2.3.4", ip, err)
	}
}

func TestRunDaemonStopsOnSIGINT(t *testing.T) {
	defer setupMocks(t, "1.2.3.4", http.StatusOK)()

	stateFile := filepath.Join(t.TempDir(), "ip")

	done := make(chan error, 1)
	go func() {
		done <- runDaemon("tok", "usr", "title", stateFile, 1*time.Hour)
	}()

	time.Sleep(50 * time.Millisecond)
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runDaemon returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runDaemon did not stop after SIGINT")
	}
}

// setViper sets viper overrides for a test and restores safe defaults on cleanup.
func setViper(t *testing.T, token, user string, daemon bool, interval time.Duration, stateFile string) {
	t.Helper()
	viper.Set("pushover-token", token)
	viper.Set("pushover-user", user)
	viper.Set("daemon", daemon)
	viper.Set("interval", interval)
	viper.Set("state-file", stateFile)
	t.Cleanup(func() {
		viper.Set("pushover-token", "")
		viper.Set("pushover-user", "")
		viper.Set("daemon", false)
		viper.Set("interval", 5*time.Minute)
		viper.Set("state-file", "")
	})
}

// --- run() dispatch tests ---

func TestRunMissingUser(t *testing.T) {
	setViper(t, "tok", "", false, 5*time.Minute, t.TempDir()+"/ip")
	if err := run(rootCmd, nil); err == nil || err.Error() != "Pushover user key required (--pushover-user or PUSHOVER_USER)" {
		t.Fatalf("expected user error, got %v", err)
	}
}

func TestRunDaemonInvalidInterval(t *testing.T) {
	setViper(t, "tok", "usr", true, 0, t.TempDir()+"/ip")
	if err := run(rootCmd, nil); err == nil || err.Error() != "interval must be positive" {
		t.Fatalf("expected interval error, got %v", err)
	}
}

func TestRunOneshotDispatch(t *testing.T) {
	defer setupMocks(t, "1.2.3.4", http.StatusOK)()
	restoreExit, exitCode := interceptExit(t)
	defer restoreExit()

	stateFile := filepath.Join(t.TempDir(), "ip")
	setViper(t, "tok", "usr", false, 5*time.Minute, stateFile)

	if err := run(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
	if exitCode() != -1 {
		t.Errorf("unexpected exit on one-shot first run, code %d", exitCode())
	}
	if ip, err := readIP(stateFile); err != nil || ip != "1.2.3.4" {
		t.Errorf("state file: got %q %v, want 1.2.3.4", ip, err)
	}
}

func TestRunDaemonDispatch(t *testing.T) {
	defer setupMocks(t, "1.2.3.4", http.StatusOK)()

	stateFile := filepath.Join(t.TempDir(), "ip")
	setViper(t, "tok", "usr", true, 10*time.Millisecond, stateFile)

	done := make(chan error, 1)
	go func() { done <- run(rootCmd, nil) }()

	time.Sleep(50 * time.Millisecond)
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run (daemon) returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run (daemon) did not stop after SIGTERM")
	}
}

// --- runOnce edge cases ---

func TestRunOnceStateReadError(t *testing.T) {
	defer setupMocks(t, "1.2.3.4", http.StatusOK)()

	// Place a directory where the state file should be — ReadFile on a directory
	// returns an error that is not ErrNotExist.
	stateFile := filepath.Join(t.TempDir(), "ip")
	os.Mkdir(stateFile, 0755)

	err := runOnce("tok", "usr", "title", stateFile)
	if err == nil {
		t.Fatal("expected error when state path is a directory, got nil")
	}
}

func TestRunOnceFirstRunNotifyFailure(t *testing.T) {
	defer setupMocks(t, "1.2.3.4", http.StatusInternalServerError)()
	restoreExit, exitCode := interceptExit(t)
	defer restoreExit()

	stateFile := filepath.Join(t.TempDir(), "ip")
	// Notify fails but state should still be written and exit 0 returned.
	if err := runOnce("tok", "usr", "title", stateFile); err != nil {
		t.Fatal(err)
	}
	if exitCode() != -1 {
		t.Errorf("unexpected exit code %d on first run with notify failure", exitCode())
	}
	if ip, err := readIP(stateFile); err != nil || ip != "1.2.3.4" {
		t.Errorf("state file: got %q %v, want 1.2.3.4", ip, err)
	}
}

func TestRunOnceIPChangedWriteError(t *testing.T) {
	defer setupMocks(t, "5.6.7.8", http.StatusOK)()
	restoreExit, _ := interceptExit(t)
	defer restoreExit()

	// Write old IP, then make the state file itself read-only.
	// Directory permissions only prevent file creation/deletion; making the
	// file read-only prevents os.WriteFile from opening it with O_WRONLY.
	stateFile := filepath.Join(t.TempDir(), "ip")
	writeIP(stateFile, "1.2.3.4")
	os.Chmod(stateFile, 0444)
	t.Cleanup(func() { os.Chmod(stateFile, 0644) })

	err := runOnce("tok", "usr", "title", stateFile)
	if err == nil {
		t.Fatal("expected error when state file is read-only, got nil")
	}
}

// --- checkIP tests ---

func TestCheckIPFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()
	orig := ipconfigURL
	ipconfigURL = srv.URL
	defer func() { ipconfigURL = orig }()

	stateFile := filepath.Join(t.TempDir(), "ip")
	checkIP("tok", "usr", "title", stateFile) // must not panic

	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Error("state file should not be created when fetchIP fails")
	}
}

func TestCheckIPStateReadError(t *testing.T) {
	defer setupMocks(t, "1.2.3.4", http.StatusOK)()

	// Directory at state path causes ReadFile to return a non-ErrNotExist error.
	stateFile := filepath.Join(t.TempDir(), "ip")
	os.Mkdir(stateFile, 0755)

	checkIP("tok", "usr", "title", stateFile) // must not panic
}

func TestCheckIPChanged(t *testing.T) {
	defer setupMocks(t, "5.6.7.8", http.StatusOK)()

	stateFile := filepath.Join(t.TempDir(), "ip")
	writeIP(stateFile, "1.2.3.4")

	checkIP("tok", "usr", "title", stateFile)

	if ip, err := readIP(stateFile); err != nil || ip != "5.6.7.8" {
		t.Errorf("state after IP change: got %q %v, want 5.6.7.8", ip, err)
	}
}

func TestCheckIPChangedNotifyFailure(t *testing.T) {
	defer setupMocks(t, "5.6.7.8", http.StatusInternalServerError)()

	stateFile := filepath.Join(t.TempDir(), "ip")
	writeIP(stateFile, "1.2.3.4")

	checkIP("tok", "usr", "title", stateFile)

	// Notify failure is a warning; state must still be updated.
	if ip, err := readIP(stateFile); err != nil || ip != "5.6.7.8" {
		t.Errorf("state after notify failure: got %q %v, want 5.6.7.8", ip, err)
	}
}
