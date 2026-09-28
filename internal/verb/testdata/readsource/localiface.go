package verb

import "dinah/internal/bench"

// want: Source rule

// plantLocal re-types its Source as a local interface, so the ReadFile it
// calls resolves to that interface's method and not to Source's.
func plantLocal(s bench.Source, p string) ([]byte, error) {
	return interface {
		ReadFile(string) ([]byte, error)
	}(s).ReadFile(p)
}
