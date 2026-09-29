package bench

// This file is not built into Dinah. The format census of
// dinah-637/criteria/39 adds it to package bench through go test -overlay (see
// the format-census job in .github/workflows/ci.yml), and a build that does
// not name the overlay never sees it.
//
// With DINAH_FORMAT_CENSUS naming a directory, each test process built with
// it writes one file there, census-<pid>.txt. Its first line says the process
// ran; after that, every store an opener refuses for declaring the card-unit
// format while the layout is switched off adds one line naming the test on
// whose stack the open was made. TestTheFormatCensus reads the directory
// afterwards and holds those lines to the tests that open such a store on
// purpose, which is how a test that opens a format-12 store without calling
// EnableCardUnitForTest first, and passes only because it never reads the
// refusal, is found.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

func init() {
	dir := os.Getenv("DINAH_FORMAT_CENSUS")
	if dir == "" {
		return
	}
	binary := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	path := filepath.Join(dir, fmt.Sprintf("census-%d.txt", os.Getpid()))
	var mu sync.Mutex
	record := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			panic("the format census cannot write " + path + ": " + err.Error())
		}
		defer file.Close()
		if _, err := file.WriteString(line + "\n"); err != nil {
			panic("the format census cannot write " + path + ": " + err.Error())
		}
	}
	record("process\t" + binary)
	refusedFormat = func(root string, declared int) {
		if declared != CardUnitFormat || cardUnitEnabled {
			return
		}
		record("refused\t" + censusTest() + "\t" + root)
	}
}

// censusTest names the outermost test function on the calling goroutine's
// stack, a subtest's closure answered as the test that declares it, or says
// that no test is on the stack.
func censusTest() string {
	pcs := make([]uintptr, 128)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	found := ""
	for {
		frame, more := frames.Next()
		name := frame.Function
		slash := strings.LastIndex(name, "/")
		base := name[slash+1:]
		if dot := strings.Index(base, "."); dot >= 0 {
			function := base[dot+1:]
			if strings.HasPrefix(function, "Test") {
				if cut := strings.Index(function, "."); cut >= 0 {
					function = function[:cut]
				}
				found = name[:slash+1] + base[:dot] + "." + function
			}
		}
		if !more {
			break
		}
	}
	if found == "" {
		return "(no test on the stack)"
	}
	return found
}
