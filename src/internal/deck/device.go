package deck

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
)

type Device struct {
	file  *os.File
	path  string
	model Model
	mu    sync.Mutex
}

func Open(path string, modelHint string) (*Device, error) {
	var err error
	var model Model
	hint := NormalizeModelID(modelHint)
	if path == "" || path == "auto" {
		path, model, err = FindDevice(hint)
		if err != nil {
			return nil, err
		}
	} else {
		model, err = ModelForDevicePath(path)
		if err != nil {
			if hint == "auto" {
				return nil, err
			}
			var ok bool
			model, ok = ModelByID(hint)
			if !ok {
				return nil, err
			}
		}
		if hint != "auto" && model.ID != hint {
			return nil, fmt.Errorf("%s is %s, not requested model %q", path, model.ID, hint)
		}
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &Device{file: file, path: path, model: model}, nil
}

func (d *Device) Path() string {
	return d.path
}

func (d *Device) Model() Model {
	return d.model
}

func (d *Device) Close() error {
	return d.file.Close()
}

func (d *Device) SetBrightness(value byte) error {
	report := make([]byte, 32)
	switch d.model.Protocol {
	case ProtocolMini:
		report[0] = 0x05
		report[1] = 0x55
		report[2] = 0xAA
		report[3] = 0xD1
		report[4] = 0x01
		report[5] = value
	case ProtocolGeneral:
		report[0] = 0x03
		report[1] = 0x08
		report[2] = value
	default:
		return fmt.Errorf("unsupported Stream Deck protocol %q", d.model.Protocol)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return sendFeature(d.file.Fd(), report)
}

func (d *Device) ShowLogo() error {
	report := make([]byte, 32)
	switch d.model.Protocol {
	case ProtocolMini:
		report[0] = 0x0B
		report[1] = 0x63
		report[2] = 0x00
	case ProtocolGeneral:
		report[0] = 0x03
		report[1] = 0x02
	default:
		return fmt.Errorf("unsupported Stream Deck protocol %q", d.model.Protocol)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return sendFeature(d.file.Fd(), report)
}

func (d *Device) BeginImageUpload() error {
	if d.model.Protocol != ProtocolMini {
		return nil
	}
	report := make([]byte, 1024)
	report[0] = 0x02
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.file.Write(report)
	return err
}

func (d *Device) UploadKeyImage(key int, data []byte) error {
	if key < 0 || key >= d.model.KeyCount() {
		return fmt.Errorf("key %d out of range", key)
	}
	switch d.model.Protocol {
	case ProtocolMini:
		return d.uploadMiniKey(key, data)
	case ProtocolGeneral:
		return d.uploadGeneralKey(key, data)
	default:
		return fmt.Errorf("unsupported Stream Deck protocol %q", d.model.Protocol)
	}
}

func (d *Device) uploadMiniKey(key int, data []byte) error {
	const payloadOffset = 0x10
	const reportSize = 1024
	const chunkSize = reportSize - payloadOffset
	chunkIndex := byte(0)

	d.mu.Lock()
	defer d.mu.Unlock()

	for offset := 0; offset < len(data); offset += chunkSize {
		end := offset + chunkSize
		if end > len(data) {
			end = len(data)
		}
		report := make([]byte, reportSize)
		report[0] = 0x02
		report[1] = 0x01
		report[2] = chunkIndex
		report[3] = 0x00
		if end == len(data) {
			report[4] = 0x01
		}
		report[5] = byte(key + 1)
		copy(report[payloadOffset:], data[offset:end])
		if _, err := d.file.Write(report); err != nil {
			return fmt.Errorf("upload key %d chunk %d: %w", key, chunkIndex, err)
		}
		chunkIndex++
	}
	return nil
}

func (d *Device) uploadGeneralKey(key int, data []byte) error {
	const payloadOffset = 0x08
	const reportSize = 1024
	const chunkSize = reportSize - payloadOffset
	chunkIndex := uint16(0)

	d.mu.Lock()
	defer d.mu.Unlock()

	for offset := 0; offset < len(data); offset += chunkSize {
		end := offset + chunkSize
		if end > len(data) {
			end = len(data)
		}
		report := make([]byte, reportSize)
		report[0] = 0x02
		report[1] = 0x07
		report[2] = byte(key)
		if end == len(data) {
			report[3] = 0x01
		}
		binary.LittleEndian.PutUint16(report[4:6], uint16(end-offset))
		binary.LittleEndian.PutUint16(report[6:8], chunkIndex)
		copy(report[payloadOffset:], data[offset:end])
		if _, err := d.file.Write(report); err != nil {
			return fmt.Errorf("upload key %d chunk %d: %w", key, chunkIndex, err)
		}
		chunkIndex++
	}
	return nil
}

func (d *Device) WatchKeys(ctx context.Context, callback func(key int, pressed bool)) error {
	switch d.model.Protocol {
	case ProtocolMini:
		return d.watchMiniKeys(ctx, callback)
	case ProtocolGeneral:
		return d.watchGeneralKeys(ctx, callback)
	default:
		return fmt.Errorf("unsupported Stream Deck protocol %q", d.model.Protocol)
	}
}

func (d *Device) watchMiniKeys(ctx context.Context, callback func(key int, pressed bool)) error {
	keyCount := d.model.KeyCount()
	previous := make([]byte, keyCount)
	buffer := make([]byte, 65)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := d.file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				return err
			}
			return err
		}
		if n < 1+keyCount || buffer[0] != 0x01 {
			continue
		}
		for key := 0; key < keyCount; key++ {
			state := buffer[1+key]
			if state != previous[key] {
				previous[key] = state
				callback(key, state != 0)
			}
		}
	}
}

func (d *Device) watchGeneralKeys(ctx context.Context, callback func(key int, pressed bool)) error {
	keyCount := d.model.KeyCount()
	previous := make([]byte, keyCount)
	buffer := make([]byte, 512)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := d.file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				return err
			}
			return err
		}
		if n < 4 || buffer[0] != 0x01 || buffer[1] != 0x00 {
			continue
		}
		payloadLen := int(binary.LittleEndian.Uint16(buffer[2:4]))
		if payloadLen > n-4 {
			payloadLen = n - 4
		}
		if payloadLen > keyCount {
			payloadLen = keyCount
		}
		for key := 0; key < payloadLen; key++ {
			state := buffer[4+key]
			if state != previous[key] {
				previous[key] = state
				callback(key, state != 0)
			}
		}
	}
}
