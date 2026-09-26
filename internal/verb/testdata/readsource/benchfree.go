package verb

import "dinah/internal/bench"

// plantedBenchFree calls an exported free function of package bench that
// reads with os.ReadFile and does not name Disk; the file of the same name
// under seam/ declares it for this plant's run.
func (l *Library) plantedBenchFree(path string) []byte {
	return bench.PlantedReadWithoutDisk(path)
}
