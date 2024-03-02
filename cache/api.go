package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type GoogleAPI struct {
	Ctx              context.Context
	Client           *http.Client
	spreadsheetId    string
	spreadsheetRange string
}

type Entry struct {
	FileID   string `json:"id"`
	Month    string `json:"date"`
	Name     string `json:"name"`
	FileName string `json:"filename"`
	FilePath string `json:"content"`
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func initGoogleAPI(spreadsheetId string, spreadsheetRange string) *GoogleAPI {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, drive.DriveReadonlyScope, sheets.SpreadsheetsReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)
	return &GoogleAPI{ctx, client, spreadsheetId, spreadsheetRange}
}

func getEntries(googleApi *GoogleAPI) ([]Entry, error) {
	srv, err := sheets.NewService(googleApi.Ctx, option.WithHTTPClient(googleApi.Client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
		return nil, err
	}

	resp, err := srv.Spreadsheets.Values.Get(googleApi.spreadsheetId, googleApi.spreadsheetRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
		return nil, err
	}

	values := resp.Values
	rows := make([]Entry, len(values))
	for i, row := range values {
		dateString := row[0].(string)
		date, err := time.Parse("02/01/2006 15.04.05", dateString)
		if err != nil {
			log.Println("Error while parsing the following time and date string:", dateString)
			date = time.Now()
		}
		rows[i] = Entry{
			FileID:   row[3].(string)[33:],
			Month:    date.Format("2006-01"),
			Name:     row[1].(string),
			FileName: "",
			FilePath: "",
		}
	}
	return rows, nil
}

func getFile(googleApi *GoogleAPI, fileID string, CachePath string) (string, error) {
	srv, err := drive.NewService(googleApi.Ctx, option.WithHTTPClient(googleApi.Client))
	if err != nil {
		log.Fatalf("Unable to retrieve Drive client: %v", err)
	}

	fileInfo, err := srv.Files.Get(fileID).Fields("name").Do()
	if err != nil {
		return "", err
	}

	fileName := fileID + filenameSeparator + fileInfo.Name
	filePath := filepath.Join(CachePath, fileName)
	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	res, err := srv.Files.Get(fileID).Download()
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	_, err = io.Copy(out, res.Body)
	if err != nil {
		return "", err
	}

	return fileName, nil
}
