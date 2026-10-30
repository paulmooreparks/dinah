package ollamaagent

import "os"

// terminate sends nothing on Windows. The os package documents only Kill
// as supported there, so the graceful phase is a wait and nothing more,
// which is what the old client's failing SIGTERM amounted to.
func terminate(*os.Process) {}
