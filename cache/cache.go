package cache

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CachePath         = "art"
	filenameSeparator = "-_-_-_-_-_"
)

type UpdateDBPayload struct {
	LastUpdated string `json:"timestamp"`
	NewEntries  int    `json:"new"`
}

type DB struct {
	LastUpdated   time.Time
	Entries       []Entry
	cachedEntries []fileDetails
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

func handleEntry(entry *Entry, db *DB) string {
	isFileCached, fileName := isCached(db.cachedEntries, entry.FileID)

	if isFileCached {
		log.Println(entry.FileID, "is cached.")
		return fileName
	}
	log.Println(entry.FileID, "is not cached. Downloading.")
	fileName, err := getFile(&db.googleApi, entry.FileID, CachePath)
	if err != nil {
		log.Println("Could not download file", entry.FileID)
		return fileName
	}
	extension := filepath.Ext(fileName)
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
	db := &DB{time.Now(), []Entry{}, files, *googleApi}
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

	total := 0
	for _, e := range res {
		if cached, _ := isCached(db.cachedEntries, e.FileID); !cached {
			total++
		}
	}

	done := 0
	if progress != nil {
		progress(done, total)
	}
	for i := range res {
		e := &res[i]
		cached, _ := isCached(db.cachedEntries, e.FileID)
		fileName := handleEntry(e, db)
		e.FilePath = filepath.Join(CachePath, fileName)
		if parts := strings.SplitN(fileName, filenameSeparator, 2); len(parts) == 2 {
			e.FileName = parts[1]
		}
		if !cached {
			done++
			if progress != nil {
				progress(done, total)
			}
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
	db.cachedEntries = []fileDetails{}
	return nil
}
