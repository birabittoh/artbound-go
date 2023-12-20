package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var templatesDirectory = "templates/"
var indexTemplate = template.Must(template.ParseFiles(templatesDirectory + "index.html"))
var helpTemplate = template.Must(template.ParseFiles(templatesDirectory + "help.html"))

type TemplateData struct {
	Emoji        EmojiDict
	LastUpdated  string
	CurrentMonth string
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	switch method := r.Method; method {
	case http.MethodGet:
		// render template
		lastUpdated := "last updated"
		currentMonth := time.Now().Format("2006-01")
		templateData := &TemplateData{defaultEmojis, lastUpdated, currentMonth}
		buf := &bytes.Buffer{}
		err := indexTemplate.Execute(buf, templateData)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		buf.WriteTo(w)
	case http.MethodPost:
		// render json
		contentType := r.Header.Get("Content-Type")
		err := r.ParseForm()
		if err != nil {
			log.Fatal("Could not parse form.")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Println(contentType, r.Form.Get("month"))
		http.Error(w, "WIP.", http.StatusMethodNotAllowed)
		return
	default:
		http.Error(w, "Please use GET or POST.", http.StatusMethodNotAllowed)
		return
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

func clearHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Please use POST.", http.StatusMethodNotAllowed)
		return
	}

	// TODO: actually clear cache
	http.Error(w, "Done.", http.StatusOK)
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

	fs := http.FileServer(http.Dir("./static"))

	r := http.NewServeMux()
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/help", helpHandler)
	r.HandleFunc("/clear", clearHandler)
	r.Handle("/static/", http.StripPrefix("/static/", fs))

	log.Println("Serving on port", port)
	err = http.ListenAndServe(":"+port, r)
	if err != nil {
		log.Fatal(err)
	}
}
