package resident

import "testing"

// TestIsLinkReadsTheNameSurrogateBit is part of dinah-619/criteria/20. It
// tables nameSurrogate over six tags from [MS-FSCC]'s table of reparse tags:
// a junction and a symbolic link, which set the name-surrogate bit, and three
// cloud placeholders and OneDrive's tag, which do not.
//
// Arming: testing bit 28 instead answers true for the cloud tags and false
// for the junction.
func TestIsLinkReadsTheNameSurrogateBit(t *testing.T) {
	rows := []struct {
		name string
		tag  uint32
		want bool
	}{
		{"IO_REPARSE_TAG_MOUNT_POINT, a junction", 0xA0000003, true},
		{"IO_REPARSE_TAG_SYMLINK", 0xA000000C, true},
		{"IO_REPARSE_TAG_CLOUD", 0x9000001A, false},
		{"IO_REPARSE_TAG_CLOUD_1", 0x9000101A, false},
		{"IO_REPARSE_TAG_CLOUD_F", 0x9000F01A, false},
		{"IO_REPARSE_TAG_ONEDRIVE", 0x80000021, false},
	}
	if len(rows) != 6 {
		t.Fatalf("the table holds %d tags, wanted 6", len(rows))
	}
	for _, r := range rows {
		if got := nameSurrogate(r.tag); got != r.want {
			t.Errorf("nameSurrogate(%#x), %s, answered %v, wanted %v", r.tag, r.name, got, r.want)
		}
	}
}
