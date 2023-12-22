package main

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
	cachedEntries []FileDetails
	googleApi     GoogleAPI
}

type FileDetails struct {
	FileName  string
	Extension string
}

func initDB(googleApi *GoogleAPI) *DB {
	files, err := listCachedEntries(cachePath)
	if err != nil {
		log.Fatal("Could not list cached entries.")
	}
	db := &DB{time.Now(), []Entry{}, files, *googleApi}
	db.update()
	return db
}

func (db *DB) update() (error, int) {
	entries, err := getEntries(&db.googleApi)
	if err != nil {
		log.Fatal("Could not update DB!", err)
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
		log.Fatal("Could not update DB!", err)
		newEntries = 0
	}
	return UpdateDBPayload{db.LastUpdated.Format("02/01/2006 15:04"), newEntries}
}

func filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func listCachedEntries(cachePath string) ([]FileDetails, error) {
	var files []FileDetails

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

		file := FileDetails{
			FileName:  fileNameWithoutExt,
			Extension: extension,
		}

		files = append(files, file)
		return nil
	})

	if err != nil {
		return []FileDetails{}, err
	}

	return files, nil
}

func isCached(cachedEntries []FileDetails, target string) (bool, string) {
	for _, file := range cachedEntries {
		if file.FileName == target {
			return true, file.Extension
		}
	}
	return false, ""
}

func (db *DB) GetEntries(month string) ([]Entry, error) {
	monthTest := func(f Entry) bool { return f.Month == month }
	res := filter(db.Entries, monthTest)

	if err := os.MkdirAll(cachePath, os.ModePerm); err != nil {
		return nil, err
	}

	for i := range res {
		e := &res[i]
		isFileCached, ext := isCached(db.cachedEntries, e.FileID)
		if isFileCached {
			log.Println(e.FileID, "is cached.")
			e.FilePath = filepath.Join(cachePath, e.FileID+ext)
		} else {
			log.Println(e.FileID, "is not cached. Downloading.")
			ext, err := getFile(&db.googleApi, e.FileID, cachePath)
			if err != nil {
				log.Fatal("Could not download file", e.FileID)
			}
			e.FilePath = filepath.Join(cachePath, e.FileID+ext)
		}
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
	db.cachedEntries = []FileDetails{}
	return nil
}
