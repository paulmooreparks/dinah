package verb

import (
	"reflect"

	"dinah/internal/bench"
)

// plantReflect takes a bench's source as any from its companion under seam/
// and calls ReadFile through reflect. No Source-shaped type appears in any
// static type here, and reflect is not judged, so this passes. It is kept as
// the reproduction of a gap the guard states rather than closes.
func plantReflect(b *bench.Bench, p string) []reflect.Value {
	return reflect.ValueOf(b.PlantAny()).MethodByName("ReadFile").Call([]reflect.Value{reflect.ValueOf(p)})
}
