package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/BiRabittoh/artbound-go/cache"
	"github.com/joho/godotenv"
)

const templatesDirectory = "templates/"

var (
	//go:embed templates/index.html templates/help.html
	templates embed.FS
	//go:embed all:static
	static        embed.FS
	indexTemplate = template.Must(template.ParseFS(templates, templatesDirectory+"index.html"))
	helpTemplate  = template.Must(template.ParseFS(templates, templatesDirectory+"help.html"))
)

type TemplateData struct {
	Emoji        EmojiDict
	LastUpdated  string
	CurrentMonth string
}

func indexHandler(db *cache.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Please use GET.", http.StatusMethodNotAllowed)
			return
		}

		lastUpdated := db.LastUpdated.Format("02/01/2006 15:04")
		currentMonth := time.Now().Format("2006-01")
		templateData := &TemplateData{defaultEmojis, lastUpdated, currentMonth}
		buf := &bytes.Buffer{}
		err := indexTemplate.Execute(buf, templateData)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		buf.WriteTo(w)
	}
}

func helpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Please use GET.", http.StatusMethodNotAllowed)
		return
	}

	buf := &bytes.Buffer{}
	err := helpTemplate.Execute(buf, defaultEmojis)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

func clearHandler(db *cache.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Please use POST.", http.StatusMethodNotAllowed)
			return
		}
		err := db.Clear()
		if err != nil {
			log.Println("Error:", err)
			http.Error(w, "Could not delete cache.", http.StatusInternalServerError)
		}
		http.Error(w, "Done.", http.StatusOK)
	}
}

func updateHandler(db *cache.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Please use POST.", http.StatusMethodNotAllowed)
			return
		}
		p := db.UpdateCall()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	}
}

func getHandler(db *cache.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := url.Parse(r.URL.String())
		if err != nil {
			log.Println("Could not parse URL.")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		month := u.Query().Get("month")
		if month == "" {
			http.Error(w, "\"month\" parameter is required.", http.StatusBadRequest)
			return
		}
		entries, err := db.GetEntries(month)
		if err != nil {
			http.Error(w, "Could not get entries.", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		if len(entries) == 0 {
			w.Write([]byte("[]"))
			return
		}
		json.NewEncoder(w).Encode(entries)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file provided.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	spreadsheetId := os.Getenv("SPREADSHEET_ID")
	if spreadsheetId == "" {
		log.Fatal("Please fill out SPREADSHEET_ID in .env")
	}

	spreadsheetRange := os.Getenv("SPREADSHEET_RANGE")
	if spreadsheetRange == "" {
		log.Fatal("Please fill out SPREADSHEET_RANGE in .env")
	}

	cacheRemotePath := "/" + cache.CachePath + "/"
	cacheFS := http.FileServer(http.Dir(cache.CachePath))

	fs := http.FileServer(http.FS(static))
	db := cache.InitDB(spreadsheetId, spreadsheetRange)

	r := http.NewServeMux()
	r.HandleFunc("/", indexHandler(db))
	r.HandleFunc("/clear", clearHandler(db))
	r.HandleFunc("/update", updateHandler(db))
	r.HandleFunc("/get", getHandler(db))
	r.HandleFunc("/help", helpHandler)
	r.Handle("/static/", fs)
	r.Handle(cacheRemotePath, http.StripPrefix(cacheRemotePath, cacheFS))

	log.Println("Serving on port", port)
	err = http.ListenAndServe(":"+port, r)
	if err != nil {
		log.Fatal(err)
	}
}
