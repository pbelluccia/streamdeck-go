//go:build linux

package main

import (
	"context"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var defaultHIDWakeupPaths = []string{"/dev", "/sys/class/hidraw"}

type hidWakeup struct {
	fd   int
	ch   chan struct{}
	once sync.Once
}

func newHIDWakeup(ctx context.Context) *hidWakeup {
	return newHIDWakeupForPaths(ctx, defaultHIDWakeupPaths)
}

func newHIDWakeupForPaths(ctx context.Context, paths []string) *hidWakeup {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	if err != nil {
		return disabledHIDWakeup()
	}

	mask := uint32(syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MOVED_TO | syscall.IN_MOVED_FROM | syscall.IN_ATTRIB | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF)
	watches := 0
	for _, path := range paths {
		if _, err := syscall.InotifyAddWatch(fd, path, mask); err == nil {
			watches++
		}
	}
	if watches == 0 {
		_ = syscall.Close(fd)
		return disabledHIDWakeup()
	}

	wakeup := &hidWakeup{
		fd: fd,
		ch: make(chan struct{}, 1),
	}
	go wakeup.readLoop()
	go func() {
		<-ctx.Done()
		wakeup.Close()
	}()
	return wakeup
}

func disabledHIDWakeup() *hidWakeup {
	return &hidWakeup{
		fd: -1,
		ch: make(chan struct{}),
	}
}

func (w *hidWakeup) C() <-chan struct{} {
	return w.ch
}

func (w *hidWakeup) Close() {
	w.once.Do(func() {
		if w.fd >= 0 {
			_ = syscall.Close(w.fd)
		}
	})
}

func (w *hidWakeup) readLoop() {
	defer w.Close()

	buffer := make([]byte, 4096)
	for {
		n, err := syscall.Read(w.fd, buffer)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return
		}
		if n <= 0 {
			continue
		}
		if bufferHasHIDEvent(buffer[:n]) {
			w.notify()
		}
	}
}

func (w *hidWakeup) notify() {
	select {
	case w.ch <- struct{}{}:
	default:
	}
}

func bufferHasHIDEvent(buffer []byte) bool {
	for offset := 0; offset+syscall.SizeofInotifyEvent <= len(buffer); {
		event := (*syscall.InotifyEvent)(unsafe.Pointer(&buffer[offset]))
		offset += syscall.SizeofInotifyEvent

		nameLen := int(event.Len)
		if nameLen < 0 || offset+nameLen > len(buffer) {
			return true
		}
		name := strings.TrimRight(string(buffer[offset:offset+nameLen]), "\x00")
		offset += nameLen

		if name == "" || strings.HasPrefix(name, "hidraw") {
			return true
		}
	}
	return false
}
