package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"

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

type changeSongRequest struct {
	Song     database.LibraryEntry `json:"song"`
	NewGroup string                `json:"new_group"`
	NewName  string                `json:"new_name"`
	NewText  string                `json:"new_text"`
	NewURL   string                `json:"new_url"`
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
	s.logger.Info("received request to delete song")
	if !s.validateRequestMethod(request.Method, http.MethodPost, writer) {
		return
	}

	song := database.LibraryEntry{}
	if !s.parseJSON(&song, writer, request) {
		return
	}

	s.writeDBResponse(s.db.DeleteSong(song), writer)
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

	s.writeDBResponse(s.db.UpdateSong(data.Song, data.NewGroup, data.NewName,
		data.NewText, data.NewURL), writer)
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
	request_url := path.Join(s.song_info_url, song_info_path)
	request_url += "?" + get_params.Encode()
	s.logger.Info("sending song info request to: ", request_url)

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

func (s *Server) writeDBResponse(err error, writer http.ResponseWriter) {
	switch err {
	case database.ErrInvalidData:
		s.logger.Error("empty group and/or song name in the request")
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
	default:
		writer.WriteHeader(http.StatusInternalServerError)
	}
}
