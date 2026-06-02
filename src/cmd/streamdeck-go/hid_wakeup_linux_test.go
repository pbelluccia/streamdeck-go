//go:build linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHIDWakeupNotifiesForHIDRawEntry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	wakeup := newHIDWakeupForPaths(ctx, []string{dir})
	defer wakeup.Close()

	if err := os.WriteFile(filepath.Join(dir, "hidraw-test"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case <-wakeup.C():
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for hidraw wakeup")
	}
}

func TestHIDWakeupIgnoresUnrelatedEntries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	wakeup := newHIDWakeupForPaths(ctx, []string{dir})
	defer wakeup.Close()

	if err := os.WriteFile(filepath.Join(dir, "unrelated"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case <-wakeup.C():
		t.Fatal("unexpected wakeup for unrelated entry")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSleepUntilDeviceRetryWakesOnHIDEvent(t *testing.T) {
	wakeups := make(chan struct{}, 1)
	wakeups <- struct{}{}

	start := time.Now()
	sleepUntilDeviceRetry(context.Background(), time.Hour, wakeups)

	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("sleepUntilDeviceRetry took %s, want immediate wakeup", elapsed)
	}
}
