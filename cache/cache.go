package cache

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	CachePath         = "art"
	filenameSeparator = "-_-_-_-_-_"
	// maximum number of files downloaded from Drive at the same time
	maxParallelDownloads = 4
)

// downloadFile is a variable so tests can replace it.
var downloadFile = getFile

type UpdateDBPayload struct {
	LastUpdated string `json:"timestamp"`
	NewEntries  int    `json:"new"`
}

type DB struct {
	LastUpdated   time.Time
	Entries       []Entry
	cachedEntries []fileDetails
	cacheMu       sync.Mutex // guards cachedEntries
	googleApi     GoogleAPI
}

type fileDetails struct {
	FileName         string
	OriginalFileName string
	Extension        string
}

func filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func listCachedEntries(CachePath string) ([]fileDetails, error) {
	var files []fileDetails

	err := filepath.Walk(CachePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil // Skip directories
		}

		fileName := info.Name()
		extension := filepath.Ext(fileName)
		fileNameWithoutExt := strings.TrimSuffix(fileName, extension)
		originalFileName := strings.Split(fileNameWithoutExt, filenameSeparator)[1]

		file := fileDetails{
			FileName:         fileNameWithoutExt,
			OriginalFileName: originalFileName,
			Extension:        extension,
		}

		files = append(files, file)
		return nil
	})

	if err != nil {
		return []fileDetails{}, err
	}

	return files, nil
}

func isCached(cachedEntries []fileDetails, target string) (bool, string) {
	for _, file := range cachedEntries {
		fileId := strings.Split(file.FileName, filenameSeparator)[0]
		if fileId == target {
			return true, file.FileName + file.Extension
		}
	}
	return false, ""
}

func (db *DB) isCached(target string) (bool, string) {
	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()
	return isCached(db.cachedEntries, target)
}

func handleEntry(entry *Entry, db *DB) string {
	isFileCached, fileName := db.isCached(entry.FileID)

	if isFileCached {
		log.Println(entry.FileID, "is cached.")
		return fileName
	}
	log.Println(entry.FileID, "is not cached. Downloading.")
	fileName, err := downloadFile(&db.googleApi, entry.FileID, CachePath)
	if err != nil {
		log.Println("Could not download file", entry.FileID)
		return fileName
	}
	extension := filepath.Ext(fileName)
	db.cacheMu.Lock()
	defer db.cacheMu.Unlock()
	db.cachedEntries = append(db.cachedEntries, fileDetails{
		FileName:  strings.TrimSuffix(fileName, extension),
		Extension: extension,
	})
	return fileName
}

func InitDB(spreadsheetId string, spreadsheetRange string) *DB {
	files, err := listCachedEntries(CachePath)
	if err != nil {
		log.Println("Could not list cached entries.")
	}
	googleApi := initGoogleAPI(spreadsheetId, spreadsheetRange)
	db := &DB{LastUpdated: time.Now(), Entries: []Entry{}, cachedEntries: files, googleApi: *googleApi}
	db.update()
	return db
}

func (db *DB) update() (int, error) {
	entries, err := getEntries(&db.googleApi)
	if err != nil {
		log.Println("Could not update DB!", err)
		return 0, err
	}
	newEntries := len(entries) - len(db.Entries)
	db.Entries = entries
	db.LastUpdated = time.Now()
	return newEntries, nil
}

func (db *DB) UpdateCall() UpdateDBPayload {
	newEntries, err := db.update()
	if err != nil {
		log.Println("Could not update DB!", err)
		newEntries = 0
	}
	return UpdateDBPayload{db.LastUpdated.Format("02/01/2006 15:04"), newEntries}
}

// GetEntries returns the entries for the given month, downloading the
// missing files. progress (optional) is called with the number of
// downloaded files and the total number of files to download.
func (db *DB) GetEntries(month string, progress func(done, total int)) ([]Entry, error) {
	monthTest := func(f Entry) bool { return f.Month == month }
	res := filter(db.Entries, monthTest)

	if err := os.MkdirAll(CachePath, os.ModePerm); err != nil {
		return nil, err
	}

	var toDownload []int
	for i, e := range res {
		if cached, _ := db.isCached(e.FileID); !cached {
			toDownload = append(toDownload, i)
		}
	}
	total := len(toDownload)

	var progressMu sync.Mutex
	done := 0
	if progress != nil {
		progress(done, total)
	}

	resolve := func(e *Entry) {
		fileName := handleEntry(e, db)
		e.FilePath = filepath.Join(CachePath, fileName)
		if parts := strings.SplitN(fileName, filenameSeparator, 2); len(parts) == 2 {
			e.FileName = parts[1]
		}
	}

	// download the missing files in parallel
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallelDownloads)
	for _, i := range toDownload {
		wg.Add(1)
		sem <- struct{}{}
		go func(e *Entry) {
			defer wg.Done()
			defer func() { <-sem }()
			resolve(e)
			progressMu.Lock()
			defer progressMu.Unlock()
			done++
			if progress != nil {
				progress(done, total)
			}
		}(&res[i])
	}
	wg.Wait()

	// the remaining entries are already cached
	for i := range res {
		if res[i].FilePath == "" {
			resolve(&res[i])
		}
	}

	return res, nil
}

func (db *DB) Clear() error {
	err := filepath.Walk(CachePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			err := os.Remove(path)
			if err != nil {
				return err
			}
			fmt.Println("Deleted:", path)
		}

		return nil
	})

	if err != nil {
		return err
	}
	db.cacheMu.Lock()
	db.cachedEntries = []fileDetails{}
	db.cacheMu.Unlock()
	return nil
}
