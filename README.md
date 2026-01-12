# httpBackupGo

httpBackupGo is a small Go application that automates downloading website backups on a schedule. It supports multiple sites, a simple web UI to manage configuration and trigger runs, and concurrent downloads with a configurable limit.

## Features

- Concurrent downloads for multiple sites (limited by configuration / env var).
- Configurable settings via a JSON config file (editable in the web UI).
- Small embedded web UI to view and change configuration and trigger runs.

## Installation

1. Clone the repository:
   ```powershell
   git clone https://github.com/yourusername/httpBackupGo.git; cd httpBackupGo
   ```

2. Build the application:
   ```powershell
   go build -o httpBackupGo.exe
   ```

3. Run the application:
   ```powershell
   .\httpBackupGo.exe
   ```

## Configuration

The application uses a JSON configuration file to manage settings. By default the config path is:

- Windows: `%ProgramData%\httpBackupGo\config.json` (if `ProgramData` is set)
- Otherwise: `config.json` next to where you run the binary

The config contains the following fields:

- `WebListenAddr` - address:port where the web UI listens (default: `127.0.0.1:8123`).
- `IntervalMinutes` - scheduler interval in minutes (default: `5`, values <= 0 are normalized to `1`).
- `BackupFolder` - base folder where backups are stored (default on Windows: `%ProgramData%\httpBackupGo\Backups`, otherwise `httpBackupGo/Backups`).
- `Retention` - number of days to keep backups (default: `30`). Note: automatic cleanup is not implemented in this release.
- `Sites` - an array of sites to back up. Each site has `Enabled`, `Name`, and `Url` fields.

### Example configuration

```json
{
  "WebListenAddr": "127.0.0.1:8123",
  "IntervalMinutes": 5,
  "BackupFolder": "httpBackupGo/Backups",
  "Retention": 30,
  "Sites": [
    {
      "Enabled": true,
      "Name": "example-site",
      "Url": "http://example.com/backup.zip"
    }
  ]
}
```

## Web Interface

Start the program and open the web UI at the configured `WebListenAddr` (default: `http://127.0.0.1:8123`).

From the UI you can:

- View and edit the JSON-backed configuration.
- Save changes (saves to the config file and notifies the running scheduler to reload the interval and site list).
- Trigger an immediate run with the "Run" button.
- Reload the scheduler manually.

The web UI endpoints used by the app are:

- `/` - index page
- `/save` - POST to save config (used by the UI)
- `/run` - POST to trigger an immediate run
- `/reload` - POST to force a scheduler/config reload

Config changes from the UI notify the main process so you do not need to restart the binary for most changes (the listen address itself requires restarting the service to take effect).

## Backup storage

Backups are saved under the configured `BackupFolder` in a subfolder per site name. Filenames follow this pattern:

`<BackupFolder>/<SiteName>/backup_<SiteName>_DD-MM-YYYY_HH-mm-ss.zip`

Example:

`C:\ProgramData\httpBackupGo\Backups\example-site\backup_example-site_12-01-2026_15-04-05.zip`

## Concurrency

The downloader runs multiple site downloads concurrently. The default maximum parallel downloads is 5. You can override this at runtime with the environment variable:

- `HTTPBACKUP_MAX_PARALLEL` - set to a positive integer to change the max parallel downloads.

## Retention / Cleanup

The configuration includes a `Retention` value, but automatic deletion/cleanup of old backups is not implemented in this release. The `retention/` package currently contains a placeholder.

## Project structure

```
httpBackupGo/
├── go.mod
├── main.go
├── README.md
├── backup/
│   └── runner.go
├── config/
│   └── config.go
├── retention/
│   └── cleanup.go  (placeholder)
└── web/
    ├── server.go
    └── static/
        ├── bootstrap.bundle.min.js
        └── bootstrap.min.css
    └── templates/
        └── index.html
```

## License

This project is licensed under the MIT License - see the `LICENSE` file for details.