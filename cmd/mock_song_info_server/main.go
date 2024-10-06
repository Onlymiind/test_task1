package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Onlymiind/test_task/internal/database"
)

type response struct {
	Text        string `json:"text"`
	ReleaseDate string `json:"release_date"`
	URL         string `json:"url"`
}

func generateSongInfo(writer http.ResponseWriter, _ *http.Request) {
	result := response{}
	verse_count := rand.UintN(11) + 1
	date := time.Date(rand.IntN(100)+1950, time.Month(rand.IntN(11)+1), rand.IntN(28)+1, 0, 0, 0, 0, time.Local)
	result.URL = "http://example.com/song"
	result.ReleaseDate = date.Format(database.DateFmt)
	first := true
	for i := 0; i < int(verse_count); i++ {
		line := strings.Repeat(strconv.Itoa(i), 16)
		line += "\n"
		if !first {
			result.Text += "\n"
		}
		first = false
		result.Text += strings.Repeat(line, i+1)
	}
	response, err := json.Marshal(result)
	if err != nil {
		fmt.Println(err.Error())
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Write(response)
}

func main() {
	http.HandleFunc("/info", generateSongInfo)
	log.Fatal(http.ListenAndServe(":7070", nil))
}
