<a id="russian"></a>

**Русский** · [English](#english)

# wdtmon4

`wdtmon4` следит за USB WatchDog, регулярно отправляет ему контрольный
сигнал и предоставляет веб-интерфейс для настройки устройства, процессов,
сети и доступа к Cloud Lite.

## Требования

- Go 1.26.0 или новее
- Node.js 20.19+ или 22.12+

## Сборка

```bash
git clone https://github.com/progerstar/wdtmon4.git
cd wdtmon4/web
nvm use                 # необязательно, если установлен nvm
npm ci
npm test
npm run build
cd ..
go test ./...
go build -trimpath -o wdtmon4 .
```

Vite собирает фронтенд в `web/build`, после чего эти файлы встраиваются в
исполняемый файл Go. Поэтому в чистой копии репозитория фронтенд нужно собрать
до запуска тестов Go или компиляции программы.

Версию, которую показывает готовая программа, можно задать при выпускной сборке
без изменения исходного кода:

```bash
go build -trimpath -ldflags "-X main.VERSION=1.3" -o wdtmon4 .
```

### Пакеты для выпуска

Скрипты в `scripts/` создают готовые пакеты и файлы `.sha256`:

```bash
# Linux: AppImage текущей архитектуры; нужен appimagetool
./scripts/package-linux.sh

# Windows x86-64 (PowerShell)
pwsh -File ./scripts/package-windows.ps1

# macOS 12+: универсальный DMG для Intel и Apple Silicon
./scripts/package-macos.sh
```

Каждый скрипт сначала выполняет `npm ci` и собирает веб-интерфейс. Для повторной
локальной упаковки готового `web/build` можно задать `SKIP_WEB_BUILD=1`.

Workflow `wdtmon4-packages` проверяет ветку `main` и pull request. Тег, точно
совпадающий с версией приложения, например `v1.3`, дополнительно публикует один
GitHub Release с AppImage, Windows ZIP, универсальным DMG и контрольными суммами.

## Запуск

```text
wdtmon4 [--headless] [--host address] [--hport port] [--cloud] [serial-port]
```

Последовательный порт указывать необязательно. Если он не задан, `wdtmon4`
выбирает первый последовательный USB-порт с подходящими идентификаторами VID:PID
из `serial.go`. Порт, указанный явно, всегда имеет приоритет.

По умолчанию веб-интерфейс доступен по адресу `127.0.0.1:8000` и открывается в
системном браузере. Флаг `--headless` отключает запуск браузера, `--host` меняет
адрес привязки, а `--hport` — порт HTTP. Если браузер открыть не удалось, сервис
продолжит работать. Флаг `--version` выводит версию сборки и завершает программу.

В HTTP API нет встроенной аутентификации пользователей и TLS. Не открывайте к
нему прямой доступ из локальной сети или интернета. Для удалённой работы без
графического интерфейса разместите сервис за обратным прокси с аутентификацией
или подключайтесь через частную сеть. Требуемый адрес привязки задайте явно с
помощью `--host`.

Файл `settings.json` хранится в текущем рабочем каталоге и может содержать токен
записи Cloud Lite. В Unix-системах файл создаётся с правами `0600`. Не раскрывайте
его содержимое и так же защищайте резервные копии.

Сетевой монитор принимает имя узла, `host:port`, `tcp://host:port` или URL с
HTTP(S). Если указан только узел, монитор проверяет TCP-порт 80.

Отправкой данных в Cloud управляет сохранённый переключатель Cloud. Флаг
`--cloud` включает отправку для текущего процесса при запуске; во время работы
сервиса настройку можно изменить через веб-интерфейс. Перед сохранением
существующего токена записи программа проверяет его, отправляя состояние в Cloud.

## Проверка

```bash
cd web
npm ci
npm test
npm run test:coverage
npm run build
cd ..
go test -race ./...
go vet ./...
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

## Лицензия

MIT. См. файл [LICENSE](LICENSE).

---

<a id="english"></a>

[Русский](#russian) · **English**

# wdtmon4

`wdtmon4` monitors a UnitX USB WatchDog, keeps its heartbeat active, and
provides a web interface for device, process, network, and Cloud Lite settings.

## Requirements

- Go 1.26.0 or newer
- Node.js 20.19+ or 22.12+

## Build

```bash
git clone https://github.com/progerstar/wdtmon4.git
cd wdtmon4/web
nvm use                 # optional, when nvm is installed
npm ci
npm test
npm run build
cd ..
go test ./...
go build -trimpath -o wdtmon4 .
```

The Vite frontend is built into `web/build` and embedded in the Go executable,
so it must be built before Go tests or compilation in a clean checkout.

Release builds can override the displayed version without editing the source:

```bash
go build -trimpath -ldflags "-X main.VERSION=1.3" -o wdtmon4 .
```

### Release packages

The scripts in `scripts/` create ready-to-distribute packages and `.sha256`
files:

```bash
# Linux: an AppImage for the current architecture; appimagetool is required
./scripts/package-linux.sh

# Windows x86-64 (PowerShell)
pwsh -File ./scripts/package-windows.ps1

# macOS 12+: a universal DMG for Intel and Apple Silicon
./scripts/package-macos.sh
```

Each script runs `npm ci` and builds the web interface first. Set
`SKIP_WEB_BUILD=1` to repackage an existing local `web/build` directory.

The `wdtmon4-packages` workflow validates `main` and pull requests. A tag that
exactly matches the application version, such as `v1.3`, also publishes one
GitHub Release containing the AppImage, Windows ZIP, universal DMG, and
checksums.

## Run

```text
wdtmon4 [--headless] [--host address] [--hport port] [--cloud] [serial-port]
```

The serial port is optional. When omitted, `wdtmon4` selects the first USB
serial port matching VID:PID in `serial.go`. An
explicit port always takes precedence.

The web interface listens on `127.0.0.1:8000` by default and opens in the
system browser. `--headless` suppresses browser launch, `--host` changes the
bind address, and `--hport` changes the HTTP port. Browser-launch failure does
not stop the service. Use `--version` to print the build version and exit.

The HTTP API has no built-in user authentication or TLS. Do not expose it
directly to a LAN or the Internet. For remote headless access, keep the service
behind an authenticated reverse proxy or a private network and opt in to the
required bind address with `--host`.

`settings.json` is stored in the current working directory and can contain a
Cloud Lite write token. On Unix systems it is created with mode `0600`; protect
the file and its backups as a secret.

The network monitor accepts a host, `host:port`, `tcp://host:port`, or an
HTTP(S) URL. A plain host checks TCP port 80.

Cloud delivery is controlled by the saved Cloud switch. `--cloud` enables it
for the current process at startup; the web interface can change it while the
service is running. Existing write tokens are verified with a Cloud state write
before they are saved.

## Verification

```bash
cd web
npm ci
npm test
npm run test:coverage
npm run build
cd ..
go test -race ./...
go vet ./...
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

## License

MIT. See [LICENSE](LICENSE).
