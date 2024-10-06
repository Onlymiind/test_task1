## Сборка
`go build cmd/server/main.go`

## Использование:
`<path/to/server binary> <path/to/ .env file>`

## Переменные конфигурации:
- ADDRESS - TCP адрес сервера
- DB_USER - пользователь базы данных
- DB_PORT - порт для подключения к базе данных
- DB_PASSWORD - пароль для подключения к базе данных
- DB_HOST - хост для подключения к базе данных
- DB_NAME - имя базы данных
- DB_MIGRATIONS_PATH - путь к директории с SQL-файлами для инициализации и миграции базы данных
- LOG_FILE - путь к файлу с логами (дефолтный - `./.log.txt`)
- SONG_INFO_URL - URL для полученя данных песни при добавлении новой песни в библиотеку (не включая пути `/info`)
## Зависимости:
- Go 1.23
- PostgreSQL 17
- pgx (`github.com/jackc/pgx`)
- godotenv (`github.com/joho/godotenv`)
- golang-migrate (`github.com/golang-migrate/migrate`)

## Примечания
- Для удобства тестирования был реализован мок-сервер для получения данных песни, команда для сборки: `go build cmd/mock_song_info_server/main.go`
