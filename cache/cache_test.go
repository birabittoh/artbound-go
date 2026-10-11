package cache

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetEntriesParallel(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(wd)

	var running, maxRunning int32
	downloadFile = func(_ *GoogleAPI, fileID string, _ string) (string, error) {
		n := atomic.AddInt32(&running, 1)
		for {
			m := atomic.LoadInt32(&maxRunning)
			if n <= m || atomic.CompareAndSwapInt32(&maxRunning, m, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		return fileID + filenameSeparator + "file.png", nil
	}
	defer func() { downloadFile = getFile }()

	db := &DB{Entries: []Entry{
		{FileID: "a", Month: "2026-10"}, {FileID: "b", Month: "2026-10"},
		{FileID: "c", Month: "2026-10"}, {FileID: "d", Month: "2026-10"},
		{FileID: "e", Month: "2026-10"}, {FileID: "f", Month: "2026-10"},
		{FileID: "x", Month: "2026-09"},
	}, cachedEntries: []fileDetails{{FileName: "f" + filenameSeparator + "old", Extension: ".gif"}}}

	var mu sync.Mutex
	var calls [][2]int
	res, err := db.GetEntries("2026-10", func(done, total int) {
		mu.Lock()
		calls = append(calls, [2]int{done, total})
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 6 {
		t.Fatalf("got %d entries", len(res))
	}
	if maxRunning < 2 || maxRunning > maxParallelDownloads {
		t.Errorf("max parallel downloads = %d", maxRunning)
	}
	if len(calls) != 6 || calls[0] != [2]int{0, 5} || calls[5] != [2]int{5, 5} {
		t.Errorf("progress calls = %v", calls)
	}
	if res[5].FileName != "old.gif" || res[0].FileName != "file.png" {
		t.Errorf("filenames = %q, %q", res[0].FileName, res[5].FileName)
	}

	// everything is cached now
	calls = nil
	db.GetEntries("2026-10", func(done, total int) { calls = append(calls, [2]int{done, total}) })
	if len(calls) != 1 || calls[0] != [2]int{0, 0} {
		t.Errorf("second run progress calls = %v", calls)
	}
}
