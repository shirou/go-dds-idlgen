package testdata_test

import (
	"encoding/binary"
	"testing"

	"github.com/shirou/go-dds-idlgen/cdr"
)

// TestRoundTrip_ManualStruct tests CDR round-trip serialization using a
// manually constructed struct that mirrors what the generator would produce.
// This validates the CDR encoder/decoder with realistic data patterns.

// SensorData mirrors the generated struct from basic_struct/input.idl.
type SensorData struct {
	SensorID    int32
	Temperature float64
	Humidity    float32
	Location    string
	Active      bool
}

func (s *SensorData) MarshalCDR(enc *cdr.Encoder) error {
	if err := enc.WriteInt32(s.SensorID); err != nil {
		return err
	}
	if err := enc.WriteFloat64(s.Temperature); err != nil {
		return err
	}
	if err := enc.WriteFloat32(s.Humidity); err != nil {
		return err
	}
	if err := enc.WriteString(s.Location); err != nil {
		return err
	}
	if err := enc.WriteBool(s.Active); err != nil {
		return err
	}
	return nil
}

func (s *SensorData) UnmarshalCDR(dec *cdr.Decoder) error {
	var err error
	if s.SensorID, err = dec.ReadInt32(); err != nil {
		return err
	}
	if s.Temperature, err = dec.ReadFloat64(); err != nil {
		return err
	}
	if s.Humidity, err = dec.ReadFloat32(); err != nil {
		return err
	}
	if s.Location, err = dec.ReadString(); err != nil {
		return err
	}
	if s.Active, err = dec.ReadBool(); err != nil {
		return err
	}
	return nil
}

func TestRoundTrip_FinalStruct(t *testing.T) {
	original := SensorData{
		SensorID:    42,
		Temperature: 23.5,
		Humidity:    65.2,
		Location:    "Building A, Room 101",
		Active:      true,
	}

	enc := cdr.NewEncoder(binary.LittleEndian)
	if err := original.MarshalCDR(enc); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	dec := cdr.NewDecoder(enc.Bytes(), binary.LittleEndian)
	var decoded SensorData
	if err := decoded.UnmarshalCDR(dec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.SensorID != original.SensorID {
		t.Errorf("SensorID: got %d, want %d", decoded.SensorID, original.SensorID)
	}
	if decoded.Temperature != original.Temperature {
		t.Errorf("Temperature: got %f, want %f", decoded.Temperature, original.Temperature)
	}
	if decoded.Humidity != original.Humidity {
		t.Errorf("Humidity: got %f, want %f", decoded.Humidity, original.Humidity)
	}
	if decoded.Location != original.Location {
		t.Errorf("Location: got %q, want %q", decoded.Location, original.Location)
	}
	if decoded.Active != original.Active {
		t.Errorf("Active: got %v, want %v", decoded.Active, original.Active)
	}
}

// AppendableEvent mirrors the appendable struct pattern with DHEADER.
type AppendableEvent struct {
	EventID   uint32
	Topic     string
	Timestamp int32
}

func (s *AppendableEvent) MarshalCDR(enc *cdr.Encoder) error {
	start := enc.BeginDHeader()
	if err := enc.WriteUint32(s.EventID); err != nil {
		return err
	}
	if err := enc.WriteString(s.Topic); err != nil {
		return err
	}
	if err := enc.WriteInt32(s.Timestamp); err != nil {
		return err
	}
	enc.FinishDHeader(start)
	return nil
}

func (s *AppendableEvent) UnmarshalCDR(dec *cdr.Decoder) error {
	_, err := dec.ReadDHeader()
	if err != nil {
		return err
	}
	if s.EventID, err = dec.ReadUint32(); err != nil {
		return err
	}
	if s.Topic, err = dec.ReadString(); err != nil {
		return err
	}
	if s.Timestamp, err = dec.ReadInt32(); err != nil {
		return err
	}
	return nil
}

func TestRoundTrip_AppendableStruct(t *testing.T) {
	original := AppendableEvent{
		EventID:   1001,
		Topic:     "sensor/temperature",
		Timestamp: 1710000000,
	}

	enc := cdr.NewEncoder(binary.LittleEndian)
	if err := original.MarshalCDR(enc); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	dec := cdr.NewDecoder(enc.Bytes(), binary.LittleEndian)
	var decoded AppendableEvent
	if err := decoded.UnmarshalCDR(dec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != original {
		t.Errorf("got %+v, want %+v", decoded, original)
	}
}

// MutableMessage mirrors the mutable struct pattern with EMHEADER per field.
type MutableMessage struct {
	MsgID   uint32
	Content string
}

func (s *MutableMessage) MarshalCDR(enc *cdr.Encoder) error {
	// Field 0: MsgID (4 bytes, fixed size)
	if err := enc.WriteEMHeader(true, 0, 4); err != nil {
		return err
	}
	if err := enc.WriteUint32(s.MsgID); err != nil {
		return err
	}
	// Field 1: Content (variable size)
	// Pre-calculate serialized size: 4 bytes (uint32 len) + len(content) + 1 (null)
	contentSize := 4 + len(s.Content) + 1
	if err := enc.WriteEMHeader(false, 1, contentSize); err != nil {
		return err
	}
	if err := enc.WriteString(s.Content); err != nil {
		return err
	}
	// Sentinel
	if err := enc.WritePLCDRSentinel(); err != nil {
		return err
	}
	return nil
}

func (s *MutableMessage) UnmarshalCDR(dec *cdr.Decoder) error {
	for {
		mu, lc, memberID, nextInt, err := dec.ReadEMHeader()
		if err != nil {
			return err
		}
		_ = mu

		if lc == 7 {
			break // sentinel
		}

		// Determine field size from LC
		var fieldSize uint32
		switch lc {
		case 0:
			fieldSize = 1
		case 1:
			fieldSize = 2
		case 2:
			fieldSize = 4
		case 3:
			fieldSize = 8
		case 4:
			fieldSize = nextInt
		case 5:
			fieldSize = nextInt * 4
		case 6:
			fieldSize = nextInt * 8
		}

		switch memberID {
		case 0:
			s.MsgID, err = dec.ReadUint32()
			if err != nil {
				return err
			}
		case 1:
			s.Content, err = dec.ReadString()
			if err != nil {
				return err
			}
		default:
			// Skip unknown field
			if err := dec.Skip(int(fieldSize)); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestRoundTrip_MutableStruct(t *testing.T) {
	original := MutableMessage{
		MsgID:   999,
		Content: "Hello, DDS World!",
	}

	enc := cdr.NewEncoder(binary.LittleEndian)
	if err := original.MarshalCDR(enc); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	dec := cdr.NewDecoder(enc.Bytes(), binary.LittleEndian)
	var decoded MutableMessage
	if err := decoded.UnmarshalCDR(dec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.MsgID != original.MsgID {
		t.Errorf("MsgID: got %d, want %d", decoded.MsgID, original.MsgID)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content: got %q, want %q", decoded.Content, original.Content)
	}
}
