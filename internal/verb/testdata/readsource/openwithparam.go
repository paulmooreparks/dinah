package verb

import "dinah/internal/bench"

// want: seam-opening rule: dinah/internal/verb.plantOpen names dinah/internal/bench.OpenWith

// plantOpen opens a workbench over a Source it is handed, naming no Disk and
// calling no method of Source.
func plantOpen(s bench.Source, root string) (*bench.Bench, error) { return bench.OpenWith(root, s) }
