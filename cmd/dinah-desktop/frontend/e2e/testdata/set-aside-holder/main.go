package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	fmt.Println("ready")
	_, _ = io.Copy(io.Discard, os.Stdin)
}
