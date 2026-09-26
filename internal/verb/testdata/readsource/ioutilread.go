package verb

import "io/ioutil"

// plantedIoutilRead reads a card's anchor through the deprecated package.
func (l *Library) plantedIoutilRead(path string) []byte {
	data, _ := ioutil.ReadFile(path)
	return data
}
