package verb

import "dinah/internal/bench"

// want: Source rule

// plantPromoted reads through a bench struct whose one field is an embedded,
// unexported alias of Source; the companion under seam/ declares it. Verb
// names neither Source nor Disk, and PlantBox is a method, not a binding
// object, but ReadFile is promoted into the struct and reads the source.
func plantPromoted(b *bench.Bench, p string) ([]byte, error) { return b.PlantBox().ReadFile(p) }
