package verb

import "dinah/internal/bench"

// want: Source rule

// plantG reads through any value whose type has Source's ReadFile.
func plantG[T interface {
	ReadFile(string) ([]byte, error)
}](t T, p string) ([]byte, error) {
	return t.ReadFile(p)
}

// plantCall hands its Source to plantG.
func plantCall(s bench.Source, p string) ([]byte, error) { return plantG(s, p) }
