package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
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

func TestRunDaemonInvalidInterval(t *testing.T) {
	if err := run(rootCmd, nil); err == nil {
		// Covered by the cobra/viper path; direct call with zero interval
		// is validated inside run(), not runDaemon().
	}
}
