package database

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/Onlymiind/test_task/internal/logger"
	"github.com/golang-migrate/migrate"
	_ "github.com/golang-migrate/migrate/database/postgres"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/jackc/pgx"
)

const (
	add_song_query                    = "add_song"
	add_song_info_query               = "add_song_info"
	add_group_query                   = "add_group"
	get_group_id_query                = "get_group"
	get_song_text_query               = "get_song_text"
	delete_song_query                 = "delete_song"
	get_library_query                 = "get_all"
	get_library_filter_base           = "SELECT name, song_name FROM groups JOIN songs ON groups.id = songs.group_id WHERE"
	get_library_filter_group_fmt      = " name LIKE $%d"
	get_library_filter_song_fmt       = " song_name LIKE $%d"
	get_library_filter_pagination_fmt = " ORDER BY name, song_name LIMIT $%d OFFSET $%d;"
)

var (
	ErrGroupNotFound = fmt.Errorf("group not found")
	ErrSongNotFound  = fmt.Errorf("song not found")
)

type Db struct {
	connection *pgx.Conn
	logger     *logger.Logger
}

func Init(user, password, host string, port uint16, db_name, migrations_path string, logger *logger.Logger) *Db {
	cfg := pgx.ConnConfig{
		User:     user,
		Password: password,
		Host:     host,
		Port:     port,
		Database: db_name,
	}

	migrations_path, err := filepath.Abs(migrations_path)
	if err != nil {
		logger.Error("failed to get an absolute path to migrations: ", err.Error())
		return nil
	}

	source_url := url.URL{Scheme: "file", Path: migrations_path}
	db_url := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     fmt.Sprintf("%s:%d", host, port),
		Path:     db_name,
		RawQuery: url.Values{"sslmode": {"disable"}}.Encode(),
	}

	logger.Info("updating database structure")
	migration, err := migrate.New(source_url.String(), db_url.String())
	if err != nil {
		logger.Error("failed to get migrations: ", err.Error())
		return nil
	}
	if err = migration.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Error("failed to migrate the database: ", err.Error())
		return nil
	}
	src_err, db_err := migration.Close()
	if src_err != nil {
		logger.Error("failed to migrate the database: ", src_err.Error())
		return nil
	} else if db_err != nil {
		logger.Error("failed to migrate the database: ", db_err.Error())
	}
	logger.Info("updating database structure: done")

	connection, err := pgx.Connect(cfg)
	if err != nil {
		return nil
	}
	logger.Info("connected to the database")

	logger.Debug("preparing queries")
	_, err = connection.Prepare(add_group_query, "INSERT INTO groups(name) VALUES ($1) RETURNING id;")
	if err != nil {
		logger.Error("failed to prepare ", add_group_query, " query: ", err.Error())
		return nil
	}
	_, err = connection.Prepare(get_group_id_query, "SELECT id FROM groups WHERE name = $1 LIMIT 1;")
	if err != nil {
		logger.Error("failed to prepare ", get_group_id_query, " query: ", err.Error())
		return nil
	}
	_, err = connection.Prepare(add_song_query, "INSERT INTO songs(group_id, song_name) VALUES($1, $2) RETURNING id;")
	if err != nil {
		logger.Error("failed to prepare ", add_song_query, " query: ", err.Error())
		return nil
	}
	_, err = connection.Prepare(add_song_info_query, "INSERT INTO song_info(song_id, lyrics, url) VALUES($1, $2, $3);")
	if err != nil {
		logger.Error("failed to prepare ", add_song_info_query, " query: ", err.Error())
		return nil
	}
	_, err = connection.Prepare(get_song_text_query, "SELECT lyrics FROM song_info WHERE song_id ="+
		" (SELECT id FROM songs WHERE group_id = $1 AND song_name = $2);")
	if err != nil {
		logger.Error("failed to prepare ", get_song_text_query, " query: ", err.Error())
		return nil
	}
	_, err = connection.Prepare(delete_song_query, "DELETE FROM songs WHERE group_id = $1 AND song_name = $2;")
	if err != nil {
		logger.Error("failed to prepare ", delete_song_query, " query: ", err.Error())
		return nil
	}
	_, err = connection.Prepare(get_library_query, "SELECT name, song_name FROM groups JOIN songs"+
		" ON groups.id = songs.group_id ORDER BY name, song_name LIMIT $1 OFFSET $2;")
	if err != nil {
		logger.Error("failed to prepare ", get_library_query, " query: ", err.Error())
		return nil
	}
	logger.Debug("preparing queries: done")

	return &Db{connection: connection, logger: logger}
}

func (db *Db) getGroupID(name string, transaction *pgx.Tx) (int64, error) {
	db.logger.Info("trying to retrieve group id, name: '", name, "'")
	rows, err := transaction.Query(get_group_id_query, name)
	defer rows.Close()
	if err != nil {
		db.logger.Error("failed to get group id: ", err.Error())
	}
	var group_id int64 = -1

	if rows.Next() {
		err = rows.Scan(&group_id)
		if err != nil {
			db.logger.Error("failed to read group id from query result: ", err.Error())
			return -1, err
		}
		return group_id, nil
	}
	return -1, nil
}

func (db *Db) getOrAddGroupID(name string, transaction *pgx.Tx) (int64, error) {
	group_id, err := db.getGroupID(name, transaction)
	if err != nil {
		return -1, err
	} else if group_id == -1 {
		db.logger.Info("group '", name, "' not found, adding it")
		rows, err := transaction.Query(add_group_query, name)
		defer rows.Close()
		if err != nil {
			db.logger.Error("failed to add new group: ", err.Error())
			return -1, err
		}
		err = rows.Scan(&group_id)
		if err != nil {
			db.logger.Error("failed to read group id from query result: ", err.Error())
			return -1, err
		}
	}
	return group_id, nil
}

func (db *Db) AddSong(group string, name string, text string, url string) error {
	db.logger.Info("adding song, group name: '", group, "' song name: '", name, "'")
	transaction, err := db.connection.Begin()
	if err != nil {
		db.logger.Error("failed to start transaction: ", err.Error())
	}
	group_id, err := db.getOrAddGroupID(group, transaction)
	if err != nil {
		db.logger.Error("failed to get group id: ", err.Error())
		transaction.Rollback()
		return err
	}
	rows, err := db.connection.Query(add_song_query, group_id, name)
	if err != nil {
		db.logger.Error("failed to add song: ", err.Error())
		transaction.Rollback()
		return err
	}
	if !rows.Next() {
		db.logger.Error("expected 1 row in insertion query result")
		transaction.Rollback()
		return fmt.Errorf("no rows after song insertion")
	}
	var song_id int64
	err = rows.Scan(&song_id)
	if err != nil {
		db.logger.Error("failed to retrieve song id from query result: ", err.Error())
		transaction.Rollback()
		return err
	}
	_, err = db.connection.Exec(add_song_info_query, song_id, text, url)
	if err != nil {
		db.logger.Error("failed to add song details: ", err.Error())
		transaction.Rollback()
		return err
	}

	err = transaction.Commit()
	if err != nil {
		db.logger.Error("failed to commit transaction: ", err.Error())
		return err
	}
	return nil
}

func (db *Db) GetSongText(group string, song string) (string, error) {
	db.logger.Info("searching for song, group: '", group, "', name: '", song, "'")
	transaction, err := db.connection.Begin()
	if err != nil {
		db.logger.Error("failed to start transaction: ", err)
		return "", err
	}
	group_id, err := db.getGroupID(group, transaction)
	if err != nil {
		return "", err
	} else if group_id == -1 {
		err = ErrGroupNotFound
		db.logger.Error(err.Error())
		transaction.Rollback()
		return "", err
	}
	rows, err := db.connection.Query(get_song_text_query, group_id, song)
	defer rows.Close()
	if err != nil {
		db.logger.Error("failed to get song text: ", err.Error())
		transaction.Rollback()
		return "", err
	}
	if !rows.Next() {
		err = ErrSongNotFound
		db.logger.Error(err.Error())
		transaction.Rollback()
		return "", err
	}
	var text string
	err = rows.Scan(text)
	if err != nil {
		db.logger.Error("failed to retrieve song text: ", err.Error())
		transaction.Rollback()
		return "", err
	}
	return text, nil
}

func (db *Db) DeleteSong(group string, song string) error {
	db.logger.Info("deleting song, group: '", group, "', song: '", song, "'")
	transaction, err := db.connection.Begin()
	if err != nil {
		db.logger.Error("failed to start transaction: ", err)
		return err
	}
	group_id, err := db.getGroupID(group, transaction)
	if err != nil {
		return err
	} else if group_id == -1 {
		err = ErrGroupNotFound
		db.logger.Error(err.Error())
		transaction.Rollback()
		return err
	}
	_, err = transaction.Exec(delete_song_query, group_id, song)
	if err != nil {
		db.logger.Error("failed to delete song: ", err.Error())
		transaction.Rollback()
		return err
	}

	err = transaction.Commit()
	if err != nil {
		db.logger.Error("failed to commit transaction: ", err.Error())
		return err
	}
	return nil
}

type LibraryEntry struct {
	GroupName string
	SongName  string
}

func (db *Db) GetAll(page_idx, page_size int) ([]LibraryEntry, error) {
	return nil, nil
}

func (db *Db) Close() {
	db.logger.Info("closing connection to the database")
	db.connection.Close()
}
