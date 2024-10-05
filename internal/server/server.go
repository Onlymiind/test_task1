package server

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Onlymiind/test_task/internal/database"
	"github.com/Onlymiind/test_task/internal/logger"
)

const (
	add_song_path    = "/add"
	get_all_path     = "/get_all"
	get_song_path    = "/get_song"
	delete_song_path = "/delete_song"
	change_song_path = "/change_song"
	song_info_path   = "/info"

	default_page_size = 20
	page_size_key     = "page_size"
	page_idx_key      = "page_idx"
	song_key          = "song"
	group_key         = "group"
	release_date_key  = "release_date"
)

var ErrWrongArgument = fmt.Errorf("wrong argument type")

type Server struct {
	db            *database.Db
	song_info_url string
	logger        *logger.Logger
}

type changeSongRequest struct {
	Song           database.LibraryEntry `json:"song"`
	NewGroup       string                `json:"new_group"`
	NewName        string                `json:"new_name"`
	NewText        string                `json:"new_text"`
	NewURL         string                `json:"new_url"`
	NewReleaseDate string                `json:"new_release_date"`
}

type songTextResponse struct {
	PageIndex int    `json:"page_idx"`
	PageCount int    `json:"page_count"`
	Verse     string `json:"verse"`
}

type songData struct {
	Text        string `json:"text"`
	ReleaseDate string `json:"release_date"`
	URL         string `json:"url"`
}

func Init(db *database.Db, song_info_url string, logger *logger.Logger) {
	server := &Server{
		db:            db,
		song_info_url: song_info_url,
		logger:        logger,
	}
	http.Handle(add_song_path, server)
	http.Handle(get_all_path, server)
	http.Handle(get_song_path, server)
	http.Handle(delete_song_path, server)
	http.Handle(change_song_path, server)
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case add_song_path:
		s.addSong(writer, request)
	case get_all_path:
		s.getAll(writer, request)
	case delete_song_path:
		s.deleteSong(writer, request)
	case change_song_path:
		s.changeSong(writer, request)
	case get_song_path:
		s.getSong(writer, request)
	default:
		writer.WriteHeader(http.StatusNotFound)
		s.logger.Error("path not found: ", request.URL.Path)
		return
	}
}

func (s *Server) getAll(writer http.ResponseWriter, request *http.Request) {
	s.logger.Info("received library retrieval request")
	if !s.validateRequestMethod(request.Method, http.MethodGet, writer) {
		return
	}

	query := request.URL.Query()
	page_idx, page_size, success := s.getPageIdxAndSize(query, writer)
	if !success {
		return
	}
	song_filter, group_filter, err := s.getSongAndGroup(query, writer)
	if err != nil {
		return
	}
	var date *time.Time
	if len(query[release_date_key]) != 0 {
		if len(query[release_date_key]) != 1 {
			s.logger.Error("expected exactly one value for ", release_date_key, " get parameter, got ", len(query[release_date_key]))
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		date_val, err := time.Parse("02.01.2006", query[release_date_key][0])
		if err != nil {
			s.logger.Error("failed to parse release date: ", err.Error())
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		date = &date_val
	}

	result, err := s.db.GetFiltered(group_filter, song_filter, page_idx, page_size, date)
	if err != nil {
		s.writeDBResponse(err, writer)
		return
	}

	result_bytes, err := json.Marshal(result)
	if err != nil {
		s.logger.Error("failed to encode library as JSON: ", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusOK)
	writer.Header().Add("Content-Type", "application/json")
	_, err = writer.Write(result_bytes)
	if err != nil {
		s.logger.Error("failed to write response: ", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.logger.Info("success")

}

func (s *Server) getSong(writer http.ResponseWriter, request *http.Request) {
	s.logger.Info("received song info retrieval request")
	if !s.validateRequestMethod(request.Method, http.MethodGet, writer) {
		return
	}

	query := request.URL.Query()
	song, group, err := s.getSongAndGroup(query, writer)
	if err != nil {
		return
	}

	text, err := s.db.GetSongText(group, song)
	if !s.writeDBResponse(err, writer) {
		return
	}

	verses := strings.Split(text, "\n\n")
	page_idx := 0
	if len(query[page_idx_key]) != 0 {
		page_idx_unsigned, success := s.parseUintGetParam(query, page_idx_key, writer)
		if !success {
			return
		} else if page_idx_unsigned > math.MaxInt {
			s.logger.Error("page index too big: ", page_idx_unsigned)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		page_idx = int(page_idx_unsigned)
	}
	if page_idx >= len(verses) {
		s.logger.Error("page index out of bounds, size: ", len(verses), ", index: ", page_idx)
	}

	result := songTextResponse{PageIndex: page_idx, PageCount: len(verses), Verse: verses[page_idx]}
	result_bytes, err := json.Marshal(result)
	if err != nil {
		s.logger.Error("failed to encode song verse as JSON: ", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusOK)
	writer.Header().Add("Content-Type", "application/json")
	_, err = writer.Write(result_bytes)
	if err != nil {
		s.logger.Error("failed to write response: ", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.logger.Info("success")
}

func (s *Server) deleteSong(writer http.ResponseWriter, request *http.Request) {
	s.logger.Info("received request to delete song")
	if !s.validateRequestMethod(request.Method, http.MethodPost, writer) {
		return
	}

	song := database.LibraryEntry{}
	if !s.parseJSON(&song, writer, request) {
		return
	}

	if s.writeDBResponse(s.db.DeleteSong(song), writer) {
		s.logger.Info("success")
	}
}

func (s *Server) changeSong(writer http.ResponseWriter, request *http.Request) {
	s.logger.Info("received request to update song details")
	if !s.validateRequestMethod(request.Method, http.MethodPost, writer) {
		return
	}

	data := changeSongRequest{}
	if !s.parseJSON(&data, writer, request) {
		return
	}

	var date *time.Time
	if data.NewReleaseDate != "" {
		date_val, err := time.Parse("02.01.2006", data.NewReleaseDate)
		if err != nil {
			s.logger.Error("failed to parse release date: ", err.Error())
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		date = &date_val
	}

	if s.writeDBResponse(s.db.UpdateSong(data.Song, data.NewGroup, data.NewName,
		data.NewText, data.NewURL, date), writer) {
		s.logger.Info("success")
	}
}

func (s *Server) addSong(writer http.ResponseWriter, request *http.Request) {
	s.logger.Info("received request to add a song to the library")
	if request.Body == nil || request.ContentLength == 0 {
		writer.WriteHeader(http.StatusBadRequest)
		s.logger.Error("missing song info")
		return
	}

	body := make([]byte, request.ContentLength)
	s.logger.Debug("add song request: length ", request.Header.Get("content-length"), " content-type ", request.Header.Get("content-type"))
	_, err := request.Body.Read(body)
	if err != io.EOF {
		writer.WriteHeader(http.StatusInternalServerError)
		s.logger.Error("failed to read request's body")
		return
	}
	song := database.LibraryEntry{}
	err = json.Unmarshal(body, &song)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		s.logger.Error("failed to parse JSON")
		return
	}

	get_params := url.Values{"group": {song.Group}, "name": {song.Song}}
	request_url := path.Join(s.song_info_url, song_info_path)
	request_url += "?" + get_params.Encode()
	s.logger.Info("sending song info request to: ", request_url)
	response, err := http.Get(request_url)
	if err != nil {
		s.logger.Error("failed to get song info: ", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	} else if response.StatusCode != http.StatusOK {
		s.logger.Error("failed to get song info, response status: ", response.Status)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	} else if response.Header.Get("content-type") != "application/json" {
		s.logger.Error("unexpected content type in response")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	body = make([]byte, response.ContentLength)
	_, err = response.Body.Read(body)
	if err != nil && err != io.EOF {
		s.logger.Error("failed to read response body: ", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	song_data := songData{}
	err = json.Unmarshal(body, &song_data)
	if err != nil {
		s.logger.Error("failed to parse the response: ", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	if song_data.Text == "" {
		s.logger.Error("song text empty")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	} else if song_data.URL == "" {
		s.logger.Error("song url empty")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	date, err := time.Parse("02.01.2006", song_data.ReleaseDate)
	if err != nil {
		s.logger.Error("failed to parse release date")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	if s.writeDBResponse(s.db.AddSong(song.Group, song.Song, song_data.Text, song_data.URL, date), writer) {
		s.logger.Info("success")
	}

}

func (s *Server) validateRequestMethod(method, expected string, writer http.ResponseWriter) bool {
	if method == expected {
		return true
	}
	s.logger.Error("request method is '", method, "', expected ", expected)
	writer.WriteHeader(http.StatusBadRequest)
	writer.Write(([]byte)("expected "))
	writer.Write(([]byte)(expected))
	return false
}

func (s *Server) parseJSON(object interface{}, writer http.ResponseWriter, request *http.Request) bool {
	s.logger.Info("parsing request data")
	body := make([]byte, request.ContentLength)
	count, err := request.Body.Read(body)
	if err != nil && err != io.EOF {
		s.logger.Error("failed to read request's body: ", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return false
	} else if count != len(body) {
		s.logger.Error("failed to fully read request's body")
		writer.WriteHeader(http.StatusInternalServerError)
		return false
	}
	err = json.Unmarshal(body, object)
	if err != nil {
		s.logger.Error("failed to parse JSON: ", err.Error())
		writer.WriteHeader(http.StatusBadRequest)
		writer.Write(([]byte)("bad JSON"))
		return false
	}

	return true
}

func (s *Server) writeDBResponse(err error, writer http.ResponseWriter) bool {
	switch err {
	case database.ErrInvalidData:
		writer.WriteHeader(http.StatusBadRequest)
		writer.Write(([]byte)("group and/or song name is empty"))
	case database.ErrGroupNotFound:
		writer.WriteHeader(http.StatusNotFound)
		writer.Write(([]byte)("non-existent group"))
	case database.ErrSongNotFound:
		writer.WriteHeader(http.StatusNotFound)
		writer.Write(([]byte)("non-existent song"))
	case nil:
		writer.WriteHeader(http.StatusOK)
		return true
	default:
		writer.WriteHeader(http.StatusInternalServerError)
	}
	return false
}

func (s *Server) parseUintGetParam(query url.Values, key string, writer http.ResponseWriter) (uint, bool) {
	if len(query[key]) != 1 {
		s.logger.Error("expected a single value for ", key, " get parameter, got: ", len(query[key]))
		writer.WriteHeader(http.StatusBadRequest)
		return 0, false
	}

	val, err := strconv.ParseUint(query[page_size_key][0], 10, 32)
	if err != nil {
		s.logger.Error("failed to parse ", key, ": ", err.Error())
		writer.WriteHeader(http.StatusBadRequest)
		return 0, false
	}
	return uint(val), true
}

func (s *Server) getPageIdxAndSize(query url.Values, writer http.ResponseWriter) (idx, size uint, success bool) {
	size = default_page_size
	if len(query[page_size_key]) != 0 {
		size, success = s.parseUintGetParam(query, page_size_key, writer)
		if !success {
			return 0, 0, false
		}
	}
	if len(query[page_idx_key]) != 0 {
		idx, success = s.parseUintGetParam(query, page_idx_key, writer)
		if !success {
			return 0, 0, false
		}
	}

	return idx, size, true
}

func (s *Server) getSongAndGroup(query url.Values, writer http.ResponseWriter) (song, group string, err error) {
	if len(query[song_key]) != 0 {
		if len(query[song_key]) != 1 {
			s.logger.Error("expected a single value for ", song_key, " get paramenter, got ", len(query[song_key]))
			writer.WriteHeader(http.StatusBadRequest)
			return "", "", ErrWrongArgument
		}
		song = query[song_key][0]
	}
	if len(query[group_key]) != 0 {
		if len(query[group_key]) != 1 {
			s.logger.Error("expected a single value for ", group_key, " get paramenter, got ", len(query[group_key]))
			writer.WriteHeader(http.StatusBadRequest)
			return "", "", ErrWrongArgument
		}
		group = query[group_key][0]
	}

	return song, group, nil
}
