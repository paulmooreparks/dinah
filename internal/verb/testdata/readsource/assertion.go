package verb

import "dinah/internal/bench"

// want: Source rule

// plantAssert asserts a value its bench companion hands out as any to an
// interface that shares ReadFile with Source and adds a method Source lacks,
// which a resident snapshot has. Source does not implement the asserted
// interface, so only the shared-method definition catches it.
func plantAssert(b *bench.Bench, p string) ([]byte, error) {
	return b.PlantAny().(interface {
		ReadFile(string) ([]byte, error)
		Generation() uint64
	}).ReadFile(p)
}
