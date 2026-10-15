package logs

import (
	"sync"
	"testing"
)

func TestGetRunningDirPrefixIsConcurrencySafe(t *testing.T) {
	var originalGetwd = getwd
	var originalStat = statFile
	var originalPrefix = runningDirPrefix
	defer func() {
		getwd = originalGetwd
		statFile = originalStat
		runningDirPrefix = originalPrefix
	}()

	getwd = func() (string, error) { return "/repo/apps/myservice", nil }
	statFile = statOnly("/repo/go.mod")
	runningDirPrefix = ""

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := getRunningDirPrefix(); got != "/repo" {
				t.Errorf("concurrent getRunningDirPrefix returned %q, want /repo", got)
			}
		}()
	}
	wg.Wait()
}
