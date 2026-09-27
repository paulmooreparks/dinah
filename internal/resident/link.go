package resident

// nameSurrogateBit is bit 29 of a reparse tag, 0x20000000.
//
// IsReparseTagNameSurrogate, which the Windows isLink answers in effect, is a
// macro whose page does not give the bit it tests, so the bit is cited to the
// two pages that do rather than to winnt.h. Microsoft's "Reparse Point Tags"
// page draws the tag's layout with bit 29 as N and says "Name surrogate bit.
// If this bit is set, the file or directory represents another named entity
// in the system." [MS-FSCC] section 2.1.2.1 gives the same bit: "N (1 bit):
// Name Surrogate bit".
//
// [MS-FSCC]'s table of reparse tags gives IO_REPARSE_TAG_MOUNT_POINT (a
// junction) as 0xA0000003 and IO_REPARSE_TAG_SYMLINK as 0xA000000C, both with
// the bit set, and IO_REPARSE_TAG_CLOUD as 0x9000001A, the cloud family
// IO_REPARSE_TAG_CLOUD_1 to IO_REPARSE_TAG_CLOUD_F as 0x9000x01A, and
// IO_REPARSE_TAG_ONEDRIVE as 0x80000021, none with it. It also says of the D
// bit, bit 28, which every cloud tag sets, "This bit MUST NOT be set when N
// (Name Surrogate) bit is set". So a workbench kept in OneDrive, whose files
// are cloud placeholders, stays held, and a junction or a symbolic link inside
// a workbench is read from disk.
const nameSurrogateBit = 0x20000000

// nameSurrogate reports whether a reparse tag's name-surrogate bit is set,
// which is what IsReparseTagNameSurrogate answers.
func nameSurrogate(tag uint32) bool {
	return tag&nameSurrogateBit != 0
}
