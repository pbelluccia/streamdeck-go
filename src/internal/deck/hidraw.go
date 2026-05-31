package deck

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

func FindDevicePath(modelHint string) (string, error) {
	path, _, err := FindDevice(modelHint)
	return path, err
}

func FindDevice(modelHint string) (string, Model, error) {
	entries, err := os.ReadDir("/sys/class/hidraw")
	if err != nil {
		return "", Model{}, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	hint := NormalizeModelID(modelHint)
	for _, entry := range entries {
		name := entry.Name()
		data, err := os.ReadFile(filepath.Join("/sys/class/hidraw", name, "device", "uevent"))
		if err != nil {
			continue
		}
		model, ok := modelFromUevent(string(data))
		if !ok {
			continue
		}
		if hint != "auto" && model.ID != hint {
			continue
		}
		return filepath.Join("/dev", name), model, nil
	}
	if hint != "auto" {
		return "", Model{}, fmt.Errorf("supported Stream Deck model %q not found", hint)
	}
	return "", Model{}, fmt.Errorf("supported Stream Deck not found")
}

func ModelForDevicePath(path string) (Model, error) {
	resolved := path
	if evaluated, err := filepath.EvalSymlinks(path); err == nil {
		resolved = evaluated
	}
	name := filepath.Base(resolved)
	if !strings.HasPrefix(name, "hidraw") {
		return Model{}, fmt.Errorf("%s is not a hidraw device", path)
	}
	data, err := os.ReadFile(filepath.Join("/sys/class/hidraw", name, "device", "uevent"))
	if err != nil {
		return Model{}, err
	}
	model, ok := modelFromUevent(string(data))
	if !ok {
		return Model{}, fmt.Errorf("%s is not a supported Stream Deck", path)
	}
	return model, nil
}

func modelFromUevent(data string) (Model, bool) {
	text := strings.ToUpper(data)
	if !strings.Contains(text, "HID_ID=") || !strings.Contains(text, VendorID+":") {
		return Model{}, false
	}
	for productID, model := range modelsByProductID {
		if strings.Contains(text, VendorID+":"+productID) {
			return model, true
		}
	}
	return Model{}, false
}

func sendFeature(fd uintptr, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty feature report")
	}
	req := hidIOCSFeature(uintptr(len(data)))
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(unsafe.Pointer(&data[0])))
	if errno != 0 {
		return errno
	}
	return nil
}

func hidIOCSFeature(length uintptr) uintptr {
	const (
		iocNRBits    = 8
		iocTypeBits  = 8
		iocSizeBits  = 14
		iocNRShift   = 0
		iocTypeShift = iocNRShift + iocNRBits
		iocSizeShift = iocTypeShift + iocTypeBits
		iocDirShift  = iocSizeShift + iocSizeBits
		iocRead      = 2
		iocWrite     = 1
	)
	return ((iocRead | iocWrite) << iocDirShift) |
		(length << iocSizeShift) |
		(uintptr('H') << iocTypeShift) |
		(0x06 << iocNRShift)
}
