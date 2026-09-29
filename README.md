# Forma API

Самостоятельный Go-сервис конвертации файлов. Фронтенд для запуска API не требуется. Структура и сборка зависимостей следуют подходу `Vault-API`.

## Структура

```text
cmd/api/main.go                     точка входа
internal/app/app.go                 зависимости, HTTP-сервер, graceful shutdown
internal/config/                   .env и настройки окружения
internal/model/                    форматы и ошибки предметной области
internal/handler/conversion/        HTTP-запросы, загрузка и скачивание
internal/handler/middleware/        CORS для отдельного фронтенда
internal/service/conversion/        выбор конвертера и жизненный цикл задания
internal/repository/files/          изолированные временные каталоги
internal/infrastructure/converter/  нативные и внешние конвертеры
scripts/smoke.py                    интеграционная проверка работающего API
docs/openapi.yaml                  описание HTTP API
```

`app` связывает файловый репозиторий, конвертеры, сервис и обработчик. Сервис зависит от интерфейсов хранилища и конвертера. Каждый запрос получает отдельную временную директорию; она удаляется после скачивания или ошибки. Постоянного хранилища и базы данных нет.

## Отдельный Docker-сервис

```sh
docker compose up --build -d
docker compose logs -f api
```

В образ включены все конвертеры. Первая сборка скачивает около 300 МБ пакетов. API опубликован на `PORT` (по умолчанию 8080). Контейнер работает от непривилегированного пользователя; временные данные находятся в tmpfs. Ограничения контейнера: 2 ГБ RAM, 2 CPU, 256 процессов.

```sh
docker compose down
```

На Linux при проблемах с сетью сборки: `BUILD_NETWORK=host docker compose build`. Это изменяет только сеть сборки.

## Конфигурация

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `HOST` | `0.0.0.0` | Интерфейс локального Go-сервера |
| `PORT` | `8080` | Локальный порт / опубликованный порт Docker |
| `CORS_ORIGINS` | localhost и 127.0.0.1 на 3000 и 3080 | Адреса фронтенда через запятую, без завершающего `/` |
| `MAX_FILE_SIZE_MB` | `50` | Максимальный размер файла |
| `CONVERSION_TIMEOUT_SECONDS` | `120` | Таймаут задания |
| `MAX_CONCURRENT_CONVERSIONS` | `2` | Число одновременно обслуживаемых конвертаций |
| `TEMP_DIR` | системная временная директория | Родительский каталог заданий при локальном запуске |
| `BUILD_NETWORK` | `default` | Сеть Docker-сборки |

Docker слушает внутри контейнера на `0.0.0.0:8080`. При увеличении лимитов файлов или времени также пересмотрите tmpfs, память и `stop_grace_period` в Compose.

CORS проверяет `Origin`, разрешает preflight и открывает `Content-Disposition`, чтобы отдельный фронтенд получал имя файла. Это политика браузерного доступа, а не аутентификация. CLI-клиенты могут работать без `Origin`.

## HTTP API

Спецификация: [docs/openapi.yaml](docs/openapi.yaml).

- `GET /api/health` — `{ "status": "ok" }`.
- `GET /api/formats` — `{ formats: [{ from, to, group }], maxFileSize, conversionTimeoutSeconds }`.
- `POST /api/convert` — multipart-поля `file` и `format`; ответ — файл с `Content-Disposition: attachment`.

Ошибки: JSON `{ "error": "..." }`. Коды: 400 — неверный запрос/формат, 403 — запрещённый Origin, 405 — метод, 413 — размер, 422 — неуспешная конвертация, 429 — сервис занят, 500 — внутренняя ошибка.

```sh
curl http://localhost:8080/api/formats
curl -F 'file=@document.pdf' -F 'format=txt' http://localhost:8080/api/convert -o document.txt
```

## Форматы

| Источник | Результат | Обработчик |
|---|---|---|
| PNG, JPG, JPEG, GIF | PNG, JPG, GIF | Go |
| Изображения, включая WebP, BMP, TIFF, AVIF | PNG, JPG, GIF, WebP, BMP, TIFF, AVIF | ImageMagick / Go |
| DOC, DOCX, ODT, RTF | PDF, DOCX, ODT, RTF, TXT | LibreOffice |
| XLS, XLSX, ODS | PDF, XLSX, ODS, CSV | LibreOffice |
| PPT, PPTX, ODP | PDF, PPTX, ODP | LibreOffice |
| TXT | PDF, DOCX, ODT | LibreOffice |
| Markdown | HTML, DOCX, ODT | Pandoc |
| PDF | TXT; PNG/JPG каждой страницы | Poppler |
| CSV ↔ JSON | JSON ↔ CSV | Go |
| MP3, WAV, OGG, FLAC, M4A, AAC | MP3, WAV, OGG, FLAC, M4A | FFmpeg |
| MP4, MOV, MKV, WebM, AVI | MP4, WebM, MP3, WAV | FFmpeg |

Одинаковые исходный и целевой форматы не предлагаются. Наличие программы не гарантирует успешную обработку повреждённого файла или любого кодека.

### Ограничения

- PDF → текст использует текстовый слой. OCR и PDF → DOCX не реализованы.
- PDF → изображения: одна страница — файл; несколько — ZIP-архив.
- GIF и многокадровые изображения: первый кадр. JPEG получает белый фон вместо прозрачности. Нативные изображения ограничены 40 мегапикселями.
- JSON → CSV: непустой массив плоских объектов; колонки сортируются, пропуски и null становятся пустыми ячейками.
- CSV → JSON: запятая как разделитель, уникальные непустые заголовки; значения сохраняются строками.
- Офисное форматирование может измениться; CSV-экспорт таблицы обычно сохраняет один лист.
- Истории на сервере и очереди заданий нет. Если все обработчики заняты, API возвращает 429.
- Для публичного развёртывания отдельно настройте HTTPS, аутентификацию, ограничение запросов и сетевую изоляцию обработчиков файлов.

## Проверки

```sh
go test ./...
go vet ./...
python3 scripts/smoke.py http://localhost:8080
```

Последняя команда требует работающего API со всеми внешними конвертерами. Проверяются CSV/JSON, изображения, DOCX/PDF, извлечение текста, ZIP страниц PDF, Markdown и аудио. Unit-тесты проверяют также CORS, ошибки, лимиты и очистку временных файлов.

## Разработка без Docker (необязательно)

Нужен Go 1.22+:

```sh
cp .env.example .env
go run ./cmd/api
```

Адрес: http://localhost:8080. Для разработки доступны `make dev`, `make build`, `make test`, `make vet`. Команда `make run` запускает Docker-сервис.

```sh
go build -o bin/converter ./cmd/api
./bin/converter
```

Запускайте из каталога backend, чтобы загрузился локальный `.env`. Системные переменные имеют приоритет над `.env`.

Для всех конвертеров на Debian/Ubuntu:

```sh
sudo apt-get update
sudo apt-get install libreoffice-writer libreoffice-calc libreoffice-impress poppler-utils imagemagick pandoc ffmpeg fonts-dejavu fonts-liberation
```

Список доступных преобразований определяется при запуске по программам в `PATH`. Без дополнительных программ доступны PNG/JPEG/GIF и CSV ↔ JSON.

# FormaBack
