package database

import (
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"github.com/Onlymiind/test_task/internal/logger"
	"github.com/golang-migrate/migrate"
	_ "github.com/golang-migrate/migrate/database/postgres"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/jackc/pgx"
)

const (
	add_song_query          = "add_song"
	add_song_info_query     = "add_song_info"
	add_group_query         = "add_group"
	get_group_id_query      = "get_group"
	get_song_text_query     = "get_song_text"
	delete_song_query       = "delete_song"
	get_library_query       = "get_all"
	get_library_count_query = "get_all_count"
	get_song_id_query       = "get_song_id"

	get_library_filter_base             = "SELECT name, song_name, release_date FROM groups JOIN songs ON groups.id = songs.group_id JOIN song_info ON songs.id = song_info.song_id WHERE"
	get_library_filter_count_base       = "SELECT COUNT(*) FROM groups JOIN songs ON groups.id = songs.group_id WHERE"
	get_library_filter_group_fmt        = " name LIKE $%d"
	get_library_filter_song_fmt         = " song_name LIKE $%d"
	get_library_filter_release_date_fmt = " release_date = $%d"
	get_library_filter_count_end        = ";"
	get_library_filter_pagination_fmt   = " ORDER BY name, song_name, release_date LIMIT $%d OFFSET $%d;"

	update_song_base             = "UPDATE songs SET"
	update_song_group_fmt        = " group_id = $%d"
	update_song_name_fmt         = " song_name = $%d"
	update_song_release_date_fmt = " release_date = $%d"
	update_song_end_fmt          = " WHERE id = $%d;"

	update_song_info_base             = "UPDATE song_info SET"
	update_song_info_text_fmt         = " lyrics = $%d"
	update_song_info_url_fmt          = " url = $%d"
	update_song_info_release_date_fmt = " release_date = $%d"
	update_song_info_end_fmt          = " WHERE song_id = $%d;"
)

var (
	ErrGroupNotFound   = fmt.Errorf("group not found")
	ErrSongNotFound    = fmt.Errorf("song not found")
	ErrEmptyFilter     = fmt.Errorf("empty filter")
	ErrInvalidData     = fmt.Errorf("invalid data")
	ErrPageOutOfBounds = fmt.Errorf("page out of bounds")
	ErrNoOutput        = fmt.Errorf("expected one row of output")
)

type Db struct {
	connection *pgx.Conn
	logger     *logger.Logger
}
type LibraryEntry struct {
	Group       string `json:"group"`
	Song        string `json:"song"`
	ReleaseDate string `json:"release_date"`
}

type LibraryPage struct {
	PageIndex uint           `json:"page_idx"`
	PageCount uint           `json:"page_count"`
	Entries   []LibraryEntry `json:"entries"`
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
	_, err = connection.Prepare(add_song_info_query, "INSERT INTO song_info(song_id, lyrics, url, release_date) VALUES($1, $2, $3, $4);")
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
	_, err = connection.Prepare(get_library_query, "SELECT name, song_name, release_date FROM groups JOIN songs"+
		" ON groups.id = songs.group_id JOIN song_info ON songs.id = song_info.song_id ORDER BY name, song_name, release_date LIMIT $1 OFFSET $2;")
	if err != nil {
		logger.Error("failed to prepare ", get_library_query, " query: ", err.Error())
		return nil
	}
	_, err = connection.Prepare(get_library_count_query, "SELECT COUNT(*) FROM groups JOIN songs"+
		" ON groups.id = songs.group_id;")
	if err != nil {
		logger.Error("failed to prepare ", get_library_count_query, " query: ", err.Error())
		return nil
	}
	_, err = connection.Prepare(get_song_id_query, "SELECT id FROM songs WHERE song_name = $1"+
		" AND group_id = (SELECT id FROM groups WHERE name = $2);")
	if err != nil {
		logger.Error("failed to prepare ", get_song_id_query, " query: ", err.Error())
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
		if !rows.Next() {
			db.logger.Error(ErrNoOutput.Error())
			return -1, ErrNoOutput
		}

		err = rows.Scan(&group_id)
		if err != nil {
			db.logger.Error("failed to read group id from query result: ", err.Error())
			return -1, err
		}
	}
	return group_id, nil
}

func (db *Db) validatePageIndex(query_result *pgx.Rows, page_idx, page_size uint) (uint, error) {
	if !query_result.Next() {
		db.logger.Error(ErrNoOutput.Error())
		return 0, ErrNoOutput
	}
	var count int64
	if err := query_result.Scan(&count); err != nil {
		db.logger.Error("failed to retrieve count: ", err.Error())
		return 0, err
	}
	max_page := count / int64(page_size)
	if count%int64(page_size) != 0 {
		max_page++
	}
	if int64(page_idx) >= max_page {
		db.logger.Error("page ", page_idx, " out of bounds, max page: ", max_page)
		return 0, ErrPageOutOfBounds
	}
	return uint(max_page) + 1, nil
}

func (db *Db) getAll(page_idx, page_size uint) (LibraryPage, error) {
	//TODO: page count
	db.logger.Info("retrieving library data, page ", page_idx, ", page size ", page_size)

	transaction, err := db.connection.Begin()
	if err != nil {
		db.logger.Error("failed to start transaction: ", err.Error())
		return LibraryPage{}, err
	}
	defer transaction.Rollback()

	// validate page index
	count_rows, err := transaction.Query(get_library_count_query)
	defer count_rows.Close()
	if err != nil {
		db.logger.Error("failed to get library entries count: ", err.Error())
		return LibraryPage{}, err
	}
	page_count, err := db.validatePageIndex(count_rows, page_idx, page_size)
	if err != nil {
		return LibraryPage{}, err
	}
	count_rows.Close()

	// get the result
	rows, err := transaction.Query(get_library_query, page_size, page_idx*page_size)
	defer rows.Close()
	if err != nil {
		db.logger.Error("failed to retrieve library: ", err.Error())
		return LibraryPage{}, err
	}
	result := LibraryPage{PageCount: page_count, PageIndex: page_idx}
	buffer := LibraryEntry{}
	time_buffer := time.Time{}
	for rows.Next() {
		err = rows.Scan(&buffer.Group, &buffer.Song, &time_buffer)
		if err != nil {
			db.logger.Error("failed to retrieve library entry: ", err.Error(), ", retrieved: ", len(result.Entries))
			return LibraryPage{}, err
		}
		buffer.ReleaseDate = time_buffer.Format("02.01.2006")
		db.logger.Debug("adding entry: group '", buffer.Group, "', song '", buffer.Song, "'")
		result.Entries = append(result.Entries, buffer)
	}

	err = transaction.Commit()
	if err != nil {
		db.logger.Error("failed to commit transaction: ", err.Error())
		return LibraryPage{}, err
	}
	return result, nil
}

func (db *Db) AddSong(group string, name string, text string, url string, date time.Time) error {
	if group == "" || name == "" || text == "" || url == "" {
		db.logger.Error("invalid use of AddSong: one of the parameters is empty")
		return ErrInvalidData
	}

	db.logger.Info("adding song, group name: '", group, "' song name: '", name, "'")
	transaction, err := db.connection.Begin()
	if err != nil {
		db.logger.Error("failed to start transaction: ", err.Error())
		return err
	}
	defer transaction.Rollback()

	group_id, err := db.getOrAddGroupID(group, transaction)
	if err != nil {
		db.logger.Error("failed to get group id: ", err.Error())
		return err
	}
	rows, err := db.connection.Query(add_song_query, group_id, name)
	defer rows.Close()
	if err != nil {
		db.logger.Error("failed to add song: ", err.Error())
		return err
	}
	if !rows.Next() {
		db.logger.Error("expected 1 row in insertion query result")
		return fmt.Errorf("no rows after song insertion")
	}
	var song_id int64
	err = rows.Scan(&song_id)
	if err != nil {
		db.logger.Error("failed to retrieve song id from query result: ", err.Error())
		return err
	}
	_, err = db.connection.Exec(add_song_info_query, song_id, text, url, date.Format("2006-01-02"))
	if err != nil {
		db.logger.Error("failed to add song details: ", err.Error())
		return err
	}

	err = transaction.Commit()
	if err != nil {
		db.logger.Error("failed to commit transaction: ", err.Error())
		return err
	}
	db.logger.Info("song successfully added")
	return nil
}

func (db *Db) GetSongText(group string, song string) (string, error) {
	if group == "" || song == "" {
		db.logger.Error("invalid use of GetSongText: one of the parameters is empty")
		return "", ErrInvalidData
	}

	db.logger.Info("searching for song, group: '", group, "', name: '", song, "'")
	transaction, err := db.connection.Begin()
	if err != nil {
		db.logger.Error("failed to start transaction: ", err)
		return "", err
	}
	defer transaction.Rollback()

	group_id, err := db.getGroupID(group, transaction)
	if err != nil {
		return "", err
	} else if group_id == -1 {
		err = ErrGroupNotFound
		db.logger.Error(err.Error())
		return "", err
	}
	rows, err := db.connection.Query(get_song_text_query, group_id, song)
	defer rows.Close()
	if err != nil {
		db.logger.Error("failed to get song text: ", err.Error())
		return "", err
	}
	if !rows.Next() {
		err = ErrSongNotFound
		db.logger.Error(err.Error())
		return "", err
	}
	var text string
	err = rows.Scan(text)
	if err != nil {
		db.logger.Error("failed to retrieve song text: ", err.Error())
		return "", err
	}

	err = transaction.Commit()
	if err != nil {
		db.logger.Error("failed to commit transaction: ", err.Error())
		return "", err
	}
	return text, nil
}

func (db *Db) DeleteSong(song LibraryEntry) error {
	if song.Group == "" || song.Song == "" {
		db.logger.Error("invalid use of DeleteSong: one of the parameters is empty")
		return ErrInvalidData
	}

	db.logger.Info("deleting song, group: '", song.Group, "', song: '", song.Song, "'")
	transaction, err := db.connection.Begin()
	if err != nil {
		db.logger.Error("failed to start transaction: ", err)
		return err
	}
	defer transaction.Rollback()

	group_id, err := db.getGroupID(song.Group, transaction)
	if err != nil {
		return err
	} else if group_id == -1 {
		err = ErrGroupNotFound
		db.logger.Error(err.Error())
		return err
	}
	_, err = transaction.Exec(delete_song_query, group_id, song.Song)
	if err != nil {
		db.logger.Error("failed to delete song: ", err.Error())
		return err
	}

	err = transaction.Commit()
	if err != nil {
		db.logger.Error("failed to commit transaction: ", err.Error())
		return err
	}
	db.logger.Info("deletion successful")
	return nil
}

func (db *Db) GetFiltered(group, song string, page_idx, page_size uint, release_date *time.Time) (LibraryPage, error) {
	db.logger.Info("retrieving filtered library data, group '", group,
		"' song '", song, "', page ", page_idx, ", page size ", page_size)
	if group == "" && song == "" {
		db.logger.Info("filter is empty")
		return db.getAll(page_idx, page_size)
	}

	query := get_library_filter_base
	count_query := get_library_filter_count_base
	arg_idx := 1
	if group != "" {
		filter_group := fmt.Sprintf(get_library_filter_group_fmt, arg_idx)
		query += filter_group
		count_query += filter_group
		if song != "" || release_date != nil {
			query += " AND"
			count_query += " AND"
		}
		arg_idx++
	}
	if song != "" {
		filter_song := fmt.Sprintf(get_library_filter_song_fmt, arg_idx)
		query += filter_song
		count_query += filter_song
		arg_idx++
		if release_date != nil {
			query += " AND"
			count_query += " AND"
		}
	}
	if release_date != nil {
		filter_date := fmt.Sprintf(get_library_filter_release_date_fmt, arg_idx)
		query += filter_date
		count_query += filter_date
		arg_idx++
	}
	query += fmt.Sprintf(get_library_filter_pagination_fmt, arg_idx, arg_idx+1)
	count_query += get_library_filter_count_end
	db.logger.Debug("resulting query: ", query)

	transaction, err := db.connection.Begin()
	if err != nil {
		db.logger.Error("failed to start transaction: ", err.Error())
		return LibraryPage{}, err
	}
	defer transaction.Rollback()

	// validate page index
	var count_rows *pgx.Rows
	if group == "" && release_date == nil {
		count_rows, err = transaction.Query(count_query, song)
	} else if song == "" && release_date == nil {
		count_rows, err = transaction.Query(query, group)
	} else if release_date == nil {
		count_rows, err = transaction.Query(query, group, song)
	} else if group == "" && song == "" {
		count_rows, err = transaction.Query(count_query, *release_date)
	} else {
		count_rows, err = transaction.Query(query, group, song, *release_date)
	}
	defer count_rows.Close()
	if err != nil {
		db.logger.Error("failed to get library entries count: ", err.Error())
		return LibraryPage{}, err
	}
	page_count, err := db.validatePageIndex(count_rows, page_idx, page_size)
	if err != nil {
		return LibraryPage{}, err
	}
	count_rows.Close()

	// get data
	var rows *pgx.Rows
	if group == "" && release_date == nil {
		rows, err = transaction.Query(query, song, page_size, page_idx*page_size)
	} else if song == "" && release_date == nil {
		rows, err = transaction.Query(query, group, page_size, page_idx*page_size)
	} else if release_date == nil {
		rows, err = transaction.Query(query, group, song, page_size, page_idx*page_size)
	} else if group == "" && song == "" {
		rows, err = transaction.Query(query, release_date, page_size, page_idx*page_size)
	} else {
		rows, err = transaction.Query(query, group, song, release_date, page_size, page_idx*page_size)
	}
	defer rows.Close()
	if err != nil {
		db.logger.Error("failed to retrieve library: ", err.Error())
		return LibraryPage{}, err
	}

	result := LibraryPage{PageCount: page_count, PageIndex: page_idx}
	buffer := LibraryEntry{}
	time_buffer := time.Time{}
	for rows.Next() {
		err = rows.Scan(&buffer.Group, &buffer.Song, &buffer.ReleaseDate, &time_buffer)
		if err != nil {
			db.logger.Error("failed to retrieve library entry: ", err.Error(), ", retrieved: ", len(result.Entries))
			return LibraryPage{}, err
		}
		buffer.ReleaseDate = time_buffer.Format("02.01.2006")
		db.logger.Debug("adding entry: group '", buffer.Group, "', song '", buffer.Song, "'")
		result.Entries = append(result.Entries, buffer)
	}

	if err = transaction.Commit(); err != nil {
		db.logger.Error("failed to commit transaction: ", err.Error())
		return LibraryPage{}, err
	}
	return result, nil
}

func (db *Db) UpdateSong(song LibraryEntry, new_group, new_name, new_text, new_url string, new_release_date *time.Time) error {
	if song.Group == "" || song.Song == "" {
		db.logger.Error("invalid use of UpdateSong: group and/or song name is empty")
		return ErrInvalidData
	} else if new_group == "" && new_name == "" && new_text == "" && new_url == "" && new_release_date == nil {
		// nothing to update
		db.logger.Debug("empty update: group '", song.Group, "', song '", song.Song, "'")
		return nil
	}

	db.logger.Info("updating song '", song.Song, "', group '", song.Group, "'")
	transaction, err := db.connection.Begin()
	if err != nil {
		db.logger.Error("failed to start transaction: ", err.Error())
		return err
	}
	defer transaction.Rollback()

	// get song id
	rows, err := transaction.Query(get_song_id_query, song.Song, song.Group)
	defer rows.Close()
	if err != nil {
		db.logger.Error("failed to get song id: ", err.Error())
		return err
	}
	if !rows.Next() {
		db.logger.Error(ErrSongNotFound.Error())
		return err
	}
	var song_id int64
	err = rows.Scan(song_id)
	if err != nil {
		db.logger.Error("failed to retrieve song id: ", err.Error())
		return err
	}

	// update group and/or song name
	if new_group != "" || new_name != "" {
		db.logger.Info("updating song name and/or group. New name: '",
			new_name, "', new group: '", new_group, "'")
		update_song_query := update_song_base
		arg_idx := 1
		var new_group_id int64
		if new_group != "" {
			db.logger.Info("new group: '", new_group, "'")
			new_group_id, err = db.getOrAddGroupID(new_group, transaction)
			if err != nil {
				db.logger.Error("failed to get new group id: ", err.Error())
				return err
			}
			update_song_query += fmt.Sprintf(update_song_group_fmt, arg_idx)
			if new_name != "" || new_release_date != nil {
				update_song_query += ","
			}
			arg_idx++
		}
		if new_name != "" {
			db.logger.Info("new song name: '", new_name, "'")
			update_song_query += fmt.Sprintf(update_song_name_fmt, arg_idx)
			if new_release_date != nil {
				update_song_query += ","
			}
			arg_idx++
		}
		if new_release_date != nil {
			db.logger.Info("new release date: ", new_release_date.Format("02.01.2006"))
			update_song_query += fmt.Sprintf(update_song_info_release_date_fmt, arg_idx)
			arg_idx++
		}
		update_song_query += fmt.Sprintf(update_song_end_fmt, arg_idx)
		db.logger.Debug("resulting query: ", update_song_query)

		if new_group == "" && new_release_date == nil {
			_, err = transaction.Exec(update_song_query, new_name, song_id)
		} else if new_name == "" && new_release_date == nil {
			_, err = transaction.Exec(update_song_query, new_group_id, song_id)
		} else if new_release_date == nil {
			_, err = transaction.Exec(update_song_query, new_group_id, new_name, song_id)
		} else if new_group == "" && new_name == "" {
			_, err = transaction.Exec(update_song_query, new_release_date, song_id)
		} else {
			_, err = transaction.Exec(update_song_query, new_group_id, new_name, new_release_date, song_id)
		}
		if err != nil {
			db.logger.Error("failed to update song: ", err.Error())
			return err
		}
	}

	//update song info
	if new_text != "" || new_url != "" {
		db.logger.Info("updating song text and/or url")
		update_song_info_query := update_song_base
		arg_idx := 1
		if new_text != "" {
			update_song_info_query += fmt.Sprintf(update_song_info_text_fmt, arg_idx)
			if new_url != "" {
				update_song_info_query += ","
			}
			arg_idx++
		}
		if new_url != "" {
			update_song_info_query += fmt.Sprintf(update_song_info_url_fmt, arg_idx)
			arg_idx++
		}
		update_song_info_query += fmt.Sprintf(update_song_info_end_fmt, arg_idx)
		db.logger.Debug("resulting query: ", update_song_info_query)

		if new_text == "" {
			_, err = transaction.Exec(update_song_info_query, new_url, song_id)
		} else if new_url == "" {
			_, err = transaction.Exec(update_song_info_query, new_text, song_id)
		} else {
			_, err = transaction.Exec(update_song_info_query, new_text, new_url, song_id)
		}
		if err != nil {
			db.logger.Error("failed to update song info: ", err.Error())
			return err
		}
	}

	err = transaction.Commit()
	if err != nil {
		db.logger.Error("failed to commit transaction: ", err.Error())
		return err
	}
	db.logger.Info("update successful")
	return nil
}

func (db *Db) Close() {
	db.logger.Info("closing connection to the database")
	db.connection.Close()
}
