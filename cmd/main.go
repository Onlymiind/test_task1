package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Onlymiind/test_task/internal/database"
	"github.com/Onlymiind/test_task/internal/logger"
	"github.com/Onlymiind/test_task/internal/server"
	"github.com/joho/godotenv"
)

const (
	default_env_file_path = "./config.env"
	default_log_file_path = "./.log.txt"

	db_user_key            = "DB_USER"
	db_password_key        = "DB_PASSWORD"
	db_host_key            = "DB_HOST"
	db_port_key            = "DB_PORT"
	db_name_key            = "DB_NAME"
	db_migrations_path_key = "DB_MIGRATIONS_PATH"
	log_file_key           = "LOG_FILE"
	song_info_url_key      = "SONG_INFO_URL"
	address_key            = "ADDRESS"
)

func main() {
	env_file_path := default_env_file_path
	if len(os.Args) > 1 {
		env_file_path = os.Args[1]
	}
	env, err := godotenv.Read(env_file_path)
	if err != nil {
		log.Fatal("failed to read the .env file ", env_file_path, ": ", err.Error())
	}

	log_file_path := env[log_file_key]
	if log_file_path == "" {
		log_file_path = default_log_file_path
	}

	log_file, err := os.OpenFile(log_file_path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		log.Fatal("failed to open log file ", log_file_path, ": ", err.Error())
	}
	defer log_file.Close()

	logger := logger.NewLogger(log_file)

	var db_port uint64 = 0
	if env[db_port_key] != "" {
		db_port, err = strconv.ParseUint(env[db_port_key], 10, 16)
		if err != nil {
			logger.Error("failed to get port number: ", err.Error())
			return
		}
	}
	if env[db_name_key] == "" {
		logger.Error("database name must be non-empty")
		return
	} else if env[db_user_key] == "" {
		logger.Error("database user must be non-empty")
		return
	} else if env[db_host_key] == "" {
		logger.Error("database host must be non-empty")
		return
	}

	db := database.Init(env[db_user_key], env[db_password_key],
		env[db_host_key], uint16(db_port), env[db_name_key], env[db_migrations_path_key], logger)
	if db == nil {
		logger.Error("failed to connect to the database")
		return
	}
	defer db.Close()

	server.Init(db, env[song_info_url_key], logger)
	logger.Info(http.ListenAndServe(env[address_key], nil).Error())
}
