package verb

import "dinah/internal/bench"

// want: Source rule

// plantRead reads through a Source it is handed, naming no Disk.
func plantRead(s bench.Source, p string) ([]byte, error) { return s.ReadFile(p) }
