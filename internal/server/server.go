package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

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
)

type Server struct {
	db            *database.Db
	song_info_url string
	logger        *logger.Logger
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
	writer.Write(([]byte)(get_all_path + "0\n"))
}

func (s *Server) getSong(writer http.ResponseWriter, request *http.Request) {
	writer.Write(([]byte)(get_song_path + "1\n"))
}

func (s *Server) deleteSong(writer http.ResponseWriter, request *http.Request) {
	writer.Write(([]byte)(delete_song_path + "2\n"))
}

func (s *Server) changeSong(writer http.ResponseWriter, request *http.Request) {
	writer.Write(([]byte)(change_song_path + "3\n"))
}

func (s *Server) addSong(writer http.ResponseWriter, request *http.Request) {
	if request.Body == nil || request.ContentLength == 0 {
		writer.WriteHeader(http.StatusBadRequest)
		s.logger.Error("missing song info")
		return
	}

	body := make([]byte, request.ContentLength)
	s.logger.Info("add song request: ", request.Header.Get("content-length"), " ", request.Header.Get("content-type"))
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
	request_url := url.URL{Scheme: "http", Host: s.song_info_url, Path: song_info_path, RawQuery: get_params.Encode()}
	s.logger.Info("sending song info request to: ", request_url.String())

}
