package bench

import "io/ioutil"

// want: ioutil.ReadFile

// plantedIoutilRead reads a file through the deprecated package.
func (b *Bench) plantedIoutilRead() []byte {
	data, _ := ioutil.ReadFile(b.Root)
	return data
}
