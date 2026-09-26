package resident

import (
	"encoding/binary"
	"errors"
	"reflect"
	"syscall"
	"testing"
	"unicode/utf16"
)

// record encodes one FILE_NOTIFY_INFORMATION record, its name as UTF-16, and
// answers it padded to a DWORD boundary with NextEntryOffset left zero.
func record(action uint32, name string) []byte {
	units := utf16.Encode([]rune(name))
	raw := make([]byte, recordHeader+2*len(units))
	binary.LittleEndian.PutUint32(raw[4:], action)
	binary.LittleEndian.PutUint32(raw[8:], uint32(2*len(units)))
	for i, unit := range units {
		binary.LittleEndian.PutUint16(raw[recordHeader+2*i:], unit)
	}
	for len(raw)%4 != 0 {
		raw = append(raw, 0)
	}
	return raw
}

// chain joins records into one buffer, setting each NextEntryOffset.
func chain(records ...[]byte) []byte {
	var buf []byte
	for i, r := range records {
		if i < len(records)-1 {
			binary.LittleEndian.PutUint32(r[0:], uint32(len(r)))
		}
		buf = append(buf, r...)
	}
	return buf
}

// TestDecodeReadsEveryRecord is part of dinah-619/criteria/7: each of the
// five actions, a buffer of several records, and a name carrying a character
// outside the Basic Multilingual Plane decode as they were encoded.
func TestDecodeReadsEveryRecord(t *testing.T) {
	names := []string{`cards\0123456789ab\card.md`, "a.txt", `d1\sub`, "old name", "new name"}
	var records [][]byte
	var want []Change
	for i, name := range names {
		records = append(records, record(uint32(i+1), name))
		want = append(want, Change{Path: name, Action: Action(i + 1)})
	}
	buf := chain(records...)
	batch, err := decode(buf, uint32(len(buf)), nil, false)
	if err != nil || batch.Overflow || !reflect.DeepEqual(batch.Changes, want) {
		t.Errorf("five records decoded as %+v, %v", batch, err)
	}
	astral := "note-\U0001F600.md"
	single := record(uint32(Modified), astral)
	batch, err = decode(single, uint32(len(single)), nil, false)
	if err != nil || len(batch.Changes) != 1 || batch.Changes[0].Path != astral {
		t.Errorf("a name outside the Basic Multilingual Plane decoded as %+v, %v", batch, err)
	}
	if len(want) != 5 {
		t.Fatalf("decoded %d actions, wanted the five", len(want))
	}
}

// TestDecodeClassifiesEveryCompletion is part of dinah-619/criteria/7: every
// row of section 4.4's table, and each way a buffer can fail to decode.
func TestDecodeClassifiesEveryCompletion(t *testing.T) {
	good := record(uint32(Added), "a.txt")
	truncated := append([]byte(nil), good...)
	binary.LittleEndian.PutUint32(truncated[8:], 400)
	misaligned := chain(record(uint32(Added), "a.txt"), record(uint32(Removed), "b.txt"))
	binary.LittleEndian.PutUint32(misaligned[0:], 18)
	badAction := record(9, "a.txt")
	failed := errors.New("the watch handle failed")
	cases := []struct {
		name     string
		buf      []byte
		n        uint32
		err      error
		closing  bool
		overflow bool
		wantErr  error
		changes  int
	}{
		{"no error and no bytes is an overflow", good, 0, nil, false, true, nil, 0},
		{"ERROR_NOTIFY_ENUM_DIR is an overflow", good, 0, syscall.Errno(1022), false, true, nil, 0},
		{"an abort while closing is closed", good, 0, syscall.Errno(995), true, false, ErrClosed, 0},
		{"an abort without closing is a failure", good, 0, syscall.Errno(995), false, false, syscall.Errno(995), 0},
		{"any other error is a failure", good, 0, failed, false, false, failed, 0},
		{"records are records", good, uint32(len(good)), nil, false, false, nil, 1},
		{"a name running past n is an overflow", truncated, uint32(len(truncated)), nil, false, true, nil, 0},
		{"a misaligned offset is an overflow", misaligned, uint32(len(misaligned)), nil, false, true, nil, 0},
		{"an action outside 1 to 5 is an overflow", badAction, uint32(len(badAction)), nil, false, true, nil, 0},
		{"n past the buffer is an overflow", good, uint32(len(good) + 100), nil, false, true, nil, 0},
	}
	for _, c := range cases {
		batch, err := decode(c.buf, c.n, c.err, c.closing)
		if !errors.Is(err, c.wantErr) && !(err == nil && c.wantErr == nil) {
			t.Errorf("%s: answered error %v, wanted %v", c.name, err, c.wantErr)
		}
		if batch.Overflow != c.overflow || len(batch.Changes) != c.changes {
			t.Errorf("%s: answered %+v, wanted overflow %v and %d changes", c.name, batch, c.overflow, c.changes)
		}
	}
	if len(cases) != 10 {
		t.Fatalf("classified %d completions, wanted ten", len(cases))
	}
}
