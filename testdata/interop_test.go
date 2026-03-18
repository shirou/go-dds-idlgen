package testdata_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/shirou/go-dds-idlgen/cdr"
)

// testdataDir returns the absolute path to the interop fixture directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "interop")
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir(t), name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// TestInterop_SensorData decodes a FINAL struct serialized by Python pycdr2.
//
// IDL:
//
//	@final
//	struct SensorData {
//	    long sensor_id;        // 42
//	    double temperature;    // 23.5
//	    float humidity;        // 65.2 (float32)
//	    string location;       // "warehouse-A"
//	    boolean active;        // true
//	};
func TestInterop_SensorData(t *testing.T) {
	data := readFixture(t, "sensor_data_le.bin")
	dec := cdr.NewDecoder(data, binary.LittleEndian)

	sensorID, err := dec.ReadInt32()
	if err != nil {
		t.Fatalf("ReadInt32 sensor_id: %v", err)
	}
	if sensorID != 42 {
		t.Errorf("sensor_id: got %d, want 42", sensorID)
	}

	temp, err := dec.ReadFloat64()
	if err != nil {
		t.Fatalf("ReadFloat64 temperature: %v", err)
	}
	if temp != 23.5 {
		t.Errorf("temperature: got %f, want 23.5", temp)
	}

	hum, err := dec.ReadFloat32()
	if err != nil {
		t.Fatalf("ReadFloat32 humidity: %v", err)
	}
	// float32 comparison: 65.2 is not exactly representable
	if hum < 65.1 || hum > 65.3 {
		t.Errorf("humidity: got %f, want ~65.2", hum)
	}

	loc, err := dec.ReadString()
	if err != nil {
		t.Fatalf("ReadString location: %v", err)
	}
	if loc != "warehouse-A" {
		t.Errorf("location: got %q, want %q", loc, "warehouse-A")
	}

	active, err := dec.ReadBool()
	if err != nil {
		t.Fatalf("ReadBool active: %v", err)
	}
	if !active {
		t.Errorf("active: got false, want true")
	}
}

// TestInterop_PrimitiveTypes decodes a struct with various primitive types
// serialized by Python pycdr2.
//
// IDL:
//
//	@final
//	struct PrimitiveTypes {
//	    boolean val_bool;    // false
//	    long val_int32;      // -12345
//	    float val_float32;   // 3.14 (float32)
//	    double val_float64;  // 2.718281828459045
//	    string val_string;   // "Hello, DDS!"
//	};
func TestInterop_PrimitiveTypes(t *testing.T) {
	data := readFixture(t, "primitive_types_le.bin")
	dec := cdr.NewDecoder(data, binary.LittleEndian)

	valBool, err := dec.ReadBool()
	if err != nil {
		t.Fatalf("ReadBool: %v", err)
	}
	if valBool {
		t.Errorf("val_bool: got true, want false")
	}

	valInt32, err := dec.ReadInt32()
	if err != nil {
		t.Fatalf("ReadInt32: %v", err)
	}
	if valInt32 != -12345 {
		t.Errorf("val_int32: got %d, want -12345", valInt32)
	}

	valFloat32, err := dec.ReadFloat32()
	if err != nil {
		t.Fatalf("ReadFloat32: %v", err)
	}
	// float32(3.14) = 3.140000104904175
	if valFloat32 < 3.13 || valFloat32 > 3.15 {
		t.Errorf("val_float32: got %f, want ~3.14", valFloat32)
	}

	valFloat64, err := dec.ReadFloat64()
	if err != nil {
		t.Fatalf("ReadFloat64: %v", err)
	}
	if valFloat64 != 2.718281828459045 {
		t.Errorf("val_float64: got %.15f, want 2.718281828459045", valFloat64)
	}

	valString, err := dec.ReadString()
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if valString != "Hello, DDS!" {
		t.Errorf("val_string: got %q, want %q", valString, "Hello, DDS!")
	}
}

// TestInterop_EmptyString decodes a struct with an empty string field.
func TestInterop_EmptyString(t *testing.T) {
	data := readFixture(t, "empty_string_le.bin")
	dec := cdr.NewDecoder(data, binary.LittleEndian)

	valBool, err := dec.ReadBool()
	if err != nil {
		t.Fatalf("ReadBool: %v", err)
	}
	if !valBool {
		t.Errorf("val_bool: got false, want true")
	}

	valInt32, err := dec.ReadInt32()
	if err != nil {
		t.Fatalf("ReadInt32: %v", err)
	}
	if valInt32 != 0 {
		t.Errorf("val_int32: got %d, want 0", valInt32)
	}

	valFloat32, err := dec.ReadFloat32()
	if err != nil {
		t.Fatalf("ReadFloat32: %v", err)
	}
	if valFloat32 != 0.0 {
		t.Errorf("val_float32: got %f, want 0.0", valFloat32)
	}

	valFloat64, err := dec.ReadFloat64()
	if err != nil {
		t.Fatalf("ReadFloat64: %v", err)
	}
	if valFloat64 != 0.0 {
		t.Errorf("val_float64: got %f, want 0.0", valFloat64)
	}

	valString, err := dec.ReadString()
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if valString != "" {
		t.Errorf("val_string: got %q, want %q", valString, "")
	}
}
