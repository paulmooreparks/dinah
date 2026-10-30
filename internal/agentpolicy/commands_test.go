package agentpolicy

import "testing"

func TestReservedCommand(t *testing.T) {
	for _, line := range []string{`dinah credential read`, `dinah.exe flow answer x`, `& "C:\Program Files\Dinah\dinah.exe" credential get`, `DINAH credential list`} {
		if !ReservedCommand(line) {
			t.Errorf("reserved command accepted: %s", line)
		}
	}
	for _, line := range []string{`echo hello`, `dinah send paul hello`, `dinah flow list`, `other-cli credential get`} {
		if ReservedCommand(line) {
			t.Errorf("ordinary command refused: %s", line)
		}
	}
}

// A build lists its own programs, and each is guarded like the first.
func TestReservedPatternCoversEveryListedProgram(t *testing.T) {
	re := reservedPattern([]string{"dinah", "acme-cli", "odd.name"})
	for _, line := range []string{`acme-cli credential set`, `"C:\tools\acme-cli.exe" credential get`, `odd.name flow answer run`} {
		if !re.MatchString(line) {
			t.Errorf("listed program accepted: %s", line)
		}
	}
	for _, line := range []string{`acme-cli flow list`, `oddxname credential get`} {
		if re.MatchString(line) {
			t.Errorf("unlisted command refused: %s", line)
		}
	}
}
