package cache

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var cachePath = filepath.Join("static", "res", "cache")

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
	FileName  string
	Extension string
}

func filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func listCachedEntries(cachePath string) ([]fileDetails, error) {
	var files []fileDetails

	err := filepath.Walk(cachePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil // Skip directories
		}

		fileName := info.Name()
		extension := filepath.Ext(fileName)
		fileNameWithoutExt := strings.TrimSuffix(fileName, extension)

		file := fileDetails{
			FileName:  fileNameWithoutExt,
			Extension: extension,
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
		if file.FileName == target {
			return true, file.Extension
		}
	}
	return false, ""
}

func handleEntry(entry *Entry, db *DB) string {
	isFileCached, ext := isCached(db.cachedEntries, entry.FileID)

	if isFileCached {
		log.Println(entry.FileID, "is cached.")
		return ext
	}
	log.Println(entry.FileID, "is not cached. Downloading.")
	ext, err := getFile(&db.googleApi, entry.FileID, cachePath)
	if err != nil {
		log.Println("Could not download file", entry.FileID)
	}
	return ext
}

func InitDB(spreadsheetId string, spreadsheetRange string) *DB {
	files, err := listCachedEntries(cachePath)
	if err != nil {
		log.Println("Could not list cached entries.")
	}
	googleApi := initGoogleAPI(spreadsheetId, spreadsheetRange)
	db := &DB{time.Now(), []Entry{}, files, *googleApi}
	db.update()
	return db
}

func (db *DB) update() (error, int) {
	entries, err := getEntries(&db.googleApi)
	if err != nil {
		log.Println("Could not update DB!", err)
		return err, 0
	}
	newEntries := len(entries) - len(db.Entries)
	db.Entries = entries
	db.LastUpdated = time.Now()
	return nil, newEntries
}

func (db *DB) UpdateCall() UpdateDBPayload {
	err, newEntries := db.update()
	if err != nil {
		log.Println("Could not update DB!", err)
		newEntries = 0
	}
	return UpdateDBPayload{db.LastUpdated.Format("02/01/2006 15:04"), newEntries}
}

func (db *DB) GetEntries(month string) ([]Entry, error) {
	monthTest := func(f Entry) bool { return f.Month == month }
	res := filter(db.Entries, monthTest)

	if err := os.MkdirAll(cachePath, os.ModePerm); err != nil {
		return nil, err
	}

	for i := range res {
		e := &res[i]
		ext := handleEntry(e, db)
		e.FilePath = filepath.Join(cachePath, e.FileID+ext)
	}

	return res, nil
}

func (db *DB) Clear() error {
	err := filepath.Walk(cachePath, func(path string, info os.FileInfo, err error) error {
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
