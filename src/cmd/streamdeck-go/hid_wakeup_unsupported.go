//go:build !linux

package main

import "context"

type hidWakeup struct {
	ch chan struct{}
}

func newHIDWakeup(ctx context.Context) *hidWakeup {
	return &hidWakeup{ch: make(chan struct{})}
}

func (w *hidWakeup) C() <-chan struct{} {
	return w.ch
}

func (w *hidWakeup) Close() {
}
