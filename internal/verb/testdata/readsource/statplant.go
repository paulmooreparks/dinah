package verb

import (
	"io/fs"

	"dinah/internal/bench"
)

// want: Source rule

// plantStat asserts a value its bench companion hands out as any to an
// interface sharing only Stat with Source. Stat's signature names
// fs.FileInfo, and two named types are identical under go/types only when
// they are the same object, so this is caught only when package bench and
// this package are checked through one importer (dinah-619/questions/3).
func plantStat(b *bench.Bench, p string) (fs.FileInfo, error) {
	return b.PlantAny().(interface {
		Stat(string) (fs.FileInfo, error)
	}).Stat(p)
}
