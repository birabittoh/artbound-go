# artbound-go

A client-server reimplementation of the administration panel for ArtBound.

## Configuration
1. Copy `.env.example` into `.env` and fill it out with your Sheets ID and Range;
2. [Generate](https://developers.google.com/workspace/guides/create-credentials) a `credentials.json` with Drive and Sheets APIs and the following redirect URL: `http://localhost:3000`;
3. Use `go run .` to run the server and generate a `token.json` for the first time.

## Docker
The provided config needs the following files to be present in the main project folder:
* `.env`, for SPREADSHEET_ID and SPREADSHEET_RANGE,
* `credentials.json`, for Google API credentials,
* `token.json`, for your Google API access token.

After you've set up everything, it's as easy as:
```
docker-compose up -d
```
