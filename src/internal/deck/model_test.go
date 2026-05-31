package deck

import (
	"encoding/binary"
	"io"
	"os"
	"testing"
)

func TestModelForProductID(t *testing.T) {
	tests := []struct {
		productID string
		id        string
		keys      int
		protocol  Protocol
	}{
		{"0063", "mini", 6, ProtocolMini},
		{"0080", "classic", 15, ProtocolGeneral},
		{"008f", "xl", 32, ProtocolGeneral},
	}
	for _, tt := range tests {
		model, ok := ModelForProductID(tt.productID)
		if !ok {
			t.Fatalf("ModelForProductID(%q) not found", tt.productID)
		}
		if model.ID != tt.id || model.KeyCount() != tt.keys || model.Protocol != tt.protocol {
			t.Fatalf("ModelForProductID(%q) = %#v", tt.productID, model)
		}
	}
}

func TestUploadMiniKeyImageReport(t *testing.T) {
	file := tempDeviceFile(t)
	device := &Device{file: file, model: defaultModels["mini"]}

	if err := device.UploadKeyImage(2, []byte{0xaa, 0xbb, 0xcc}); err != nil {
		t.Fatal(err)
	}

	report := readDeviceFile(t, file)
	if len(report) != 1024 {
		t.Fatalf("report length = %d, want 1024", len(report))
	}
	if report[0] != 0x02 || report[1] != 0x01 || report[2] != 0x00 || report[4] != 0x01 || report[5] != 0x03 {
		t.Fatalf("unexpected mini header: % x", report[:16])
	}
	if got := report[16:19]; string(got) != string([]byte{0xaa, 0xbb, 0xcc}) {
		t.Fatalf("payload = % x", got)
	}
}

func TestUploadGeneralKeyImageChunks(t *testing.T) {
	file := tempDeviceFile(t)
	device := &Device{file: file, model: defaultModels["classic"]}
	payload := make([]byte, 1026)
	for i := range payload {
		payload[i] = byte(i)
	}

	if err := device.UploadKeyImage(7, payload); err != nil {
		t.Fatal(err)
	}

	data := readDeviceFile(t, file)
	if len(data) != 2048 {
		t.Fatalf("report length = %d, want 2048", len(data))
	}
	first := data[:1024]
	if first[0] != 0x02 || first[1] != 0x07 || first[2] != 0x07 || first[3] != 0x00 {
		t.Fatalf("unexpected first general header: % x", first[:8])
	}
	if got := binary.LittleEndian.Uint16(first[4:6]); got != 1016 {
		t.Fatalf("first payload size = %d, want 1016", got)
	}
	if got := binary.LittleEndian.Uint16(first[6:8]); got != 0 {
		t.Fatalf("first chunk index = %d, want 0", got)
	}

	second := data[1024:]
	if second[0] != 0x02 || second[1] != 0x07 || second[2] != 0x07 || second[3] != 0x01 {
		t.Fatalf("unexpected second general header: % x", second[:8])
	}
	if got := binary.LittleEndian.Uint16(second[4:6]); got != 10 {
		t.Fatalf("second payload size = %d, want 10", got)
	}
	if got := binary.LittleEndian.Uint16(second[6:8]); got != 1 {
		t.Fatalf("second chunk index = %d, want 1", got)
	}
}

func tempDeviceFile(t *testing.T) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "device")
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func readDeviceFile(t *testing.T, file *os.File) []byte {
	t.Helper()
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
