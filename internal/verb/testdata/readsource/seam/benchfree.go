package bench

import "os"

// PlantedReadWithoutDisk reads a file itself, naming no Disk, which is the
// shape the first form of the library guard could not see.
func PlantedReadWithoutDisk(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}
