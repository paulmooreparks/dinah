package bench

// want: bottom rule: dinah/internal/bench.ReadText

// plantedHelper reaches the free ReadText through a helper of its own.
func (b *Bench) plantedHelper() string {
	return plantedHelperText(b.Root)
}

func plantedHelperText(path string) string {
	text, _ := ReadText(path)
	return text
}
