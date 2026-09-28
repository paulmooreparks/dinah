package verb

// want: code outside Go: function declared without a body

// plantAsm is implemented outside Go, as an assembly routine would be.
func plantAsm(p string) ([]byte, error)

func plantReadThroughAssembly(p string) ([]byte, error) { return plantAsm(p) }
