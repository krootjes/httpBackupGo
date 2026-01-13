# 🚀 httpBackupGo

**Offline-first HTTP backup scheduler with Web UI, retention and structured logging — written in Go.**

`httpBackupGo` periodically downloads HTTP-accessible backup files (for example `backup.zip`),
stores them per site with timestamped filenames, enforces a retention policy, and exposes a
local-only web interface for configuration and control.

The application is designed to run unattended as a long-running process or service
(Windows service / systemd), while remaining fully usable from a browser on `localhost`.

---

## ✨ Features

- 🕒 **Scheduled backups** with configurable interval
- 🌐 **Offline Web UI** (no CDN, all assets embedded)
- 📁 **Per-site backup directories**
- 🗂 **Retention policy** (keep last _N_ backups per site)
- ▶️ **Run now** trigger from the UI
- 🔄 **Live scheduler reload** when config changes
- ⚡ **Parallel downloads** using goroutines with a concurrency limit
- 🧠 **Atomic downloads** using temporary files
- 📜 **Structured JSON logging** (`slog`)
- 🪟 **Windows + Linux friendly paths**
- 🔒 Web UI bound to `localhost` only

---

## 🧱 Quick Start

### Build & run

```bash
git clone https://github.com/krootjes/httpBackupGo
cd httpBackupGo
go build -o httpbackupgo
./httpbackupgo
```

### Open the Web UI

```
http://127.0.0.1:8123
```

A configuration file is created automatically on first start.

---

## ⚙️ Configuration

Configuration is stored as JSON and can be edited via the Web UI or directly on disk.

Example `config.json`:

```json
{
  "IntervalMinutes": 5,
  "BackupFolder": "C:\\Backups\\httpBackupGo",
  "Retention": 30,
  "WebListenAddr": "127.0.0.1:8123",
  "Sites": [
    {
      "Enabled": true,
      "Name": "artimo1",
      "Url": "http://localhost:81/backup.zip"
    }
  ]
}
```

### Configuration fields

- **IntervalMinutes**  
  Interval between scheduled runs (minutes).

- **BackupFolder**  
  Base directory where all backups are stored.

- **Retention**  
  Number of backups to keep per site.

- **WebListenAddr**  
  Address and port for the Web UI.  
  _Changing this requires restarting the application._

- **Sites**  
  List of backup targets.

---

## 📂 Backup Layout

Backups are written to disk as:

```
<BackupFolder>/<SiteName>/backup_<SiteName>_DD-MM-YYYY_HH-mm-ss.zip
```

Example:

```
httpBackupGo/artimo1/backup_artimo1_10-01-2026_21-22-34.zip
```

Downloads are written to a temporary `.tmp` file first and then renamed,
preventing partial or corrupt backups.

---

## 🧠 How It Works

### Scheduler
- Runs on a ticker based on `IntervalMinutes`
- Reloads configuration on every tick
- Updates its interval dynamically when the config changes
- Prevents overlapping runs using an atomic guard

### Runner
- Executes backups for all enabled sites
- Uses goroutines with a semaphore for concurrency control
- Each site runs independently
- Errors in one site do not stop others

### Retention
- Applied after each successful backup
- Keeps only the newest `Retention` backups per site
- Removes the oldest backups first
- Best-effort: retention errors never fail a backup run

### Web UI
- Fully offline (embedded Bootstrap + assets)
- Edit configuration
- Enable/disable sites
- Trigger immediate runs
- Reload scheduler without restart

---

## 📜 Logging

`httpBackupGo` uses structured JSON logging via `log/slog`.

### Log destinations

- **Windows**
  ```
  C:\ProgramData\httpBackupGo\log.json
  ```

- **Linux / macOS**
  ```
  ./log.json
  ```

Logs are also written to **stdout**, making them compatible with **journald**
when running as a systemd service.

Example log entry:

```json
{
  "time": "2026-01-13T22:41:12Z",
  "level": "INFO",
  "msg": "backup: saved",
  "site": "artimo1",
  "bytes": 7340032,
  "duration_ms": 842
}
```

---

## 🔧 Environment Variables

### HTTPBACKUP_MAX_PARALLEL

Limits the number of concurrent downloads.

```bash
HTTPBACKUP_MAX_PARALLEL=10 ./httpbackupgo
```

Default: `5`

---

## 📁 Project Structure

```
httpBackupGo/
├── backup/           Backup execution logic
│   └── runner.go
├── config/           Config load/save/validation
│   └── config.go
├── retention/        Retention cleanup logic
│   └── cleanup.go
├── web/              Web UI (handlers, templates, static assets)
│   ├── server.go
│   ├── templates/
│   └── static/
├── logging/          Structured logging (slog)
│   └── logging.go
├── main.go           Scheduler & application orchestration
├── go.mod
├── go.sum
└── README.md
```

---

## 🚫 Files Not Committed

The following are intentionally ignored:

- `config.json`
- Backup zip files
- Temporary `.tmp` files
- Log files
- IDE and OS artifacts

See `.gitignore` for details.

---

## 🛡 Design Goals

- Offline-first operation
- Minimal external dependencies
- Predictable scheduling
- Safe concurrency (no race conditions)
- Clear separation of concerns
- Production-ready defaults

---

## 📜 License

MIT License  
© 2026 krootjes