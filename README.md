# 🚀 httpBackupGo

**A lightweight, offline-first HTTP backup scheduler & runner with a web UI — written in Go.**

httpBackupGo periodically downloads HTTP-accessible backup files (such as `backup.zip`),
stores them per site with timestamped filenames, and automatically enforces a retention policy.

---

## ✨ Features

- 🕒 Scheduled backups with configurable interval
- 🌐 Offline web UI (no CDN, all assets embedded)
- 📁 Per-site backup directories
- 🗂 Retention policy (keep latest N backups per site)
- ▶️ Run backups manually via UI
- 🔄 Reload scheduler without restarting
- ⚡ Parallel downloads using goroutines
- 🧠 Live config reload (no restart needed for most changes)
- 🛠 Simple JSON-based configuration

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

The config file is created automatically on first start.

---

## ⚙️ Configuration

Configuration is stored as JSON and can be edited via the Web UI or directly on disk.

Example:

```json
{
  "IntervalMinutes": 5,
  "BackupFolder": "C:\\Backups\\httpBackupGo",
  "Retention": 30,
  "WebListenAddr": "127.0.0.1:8123",
  "Sites": [
    {
      "Enabled": true,
      "Name": "example1",
      "Url": "http://localhost:81/backup.zip"
    }
  ]
}
```

### Fields

- **IntervalMinutes**  
  Backup interval in minutes.

- **BackupFolder**  
  Base directory where backups are stored.

- **Retention**  
  Maximum number of backups to keep per site.

- **WebListenAddr**  
  Address and port for the web UI  
  (changing this requires restarting the app).

- **Sites**  
  List of backup targets.

---

## 📂 Backup Layout

Backups are stored as:

```
<BackupFolder>/<SiteName>/backup_<SiteName>_DD-MM-YYYY_HH-mm-ss.zip
```

Example:

```
httpBackupGo/artimo1/backup_artimo1_10-01-2026_21-22-34.zip
```

---

## 🧠 How It Works

### Scheduler
- Runs on a configurable interval
- Reloads configuration on every tick
- Updates interval dynamically when config changes

### Runner
- Downloads enabled sites in parallel
- Uses a configurable concurrency limit
- Writes downloads atomically using `.tmp` files

### Retention
- Keeps only the newest `Retention` backups per site
- Deletes the oldest backups first
- Runs automatically after a successful download

### Web UI
- Fully offline (Bootstrap embedded)
- Edit configuration
- Trigger backups manually
- Reload scheduler instantly

---

## 📁 Project Structure

```
httpBackupGo/
├── backup/           Download & execution logic
│   └── runner.go
├── config/           Config load/save/validation
│   └── config.go
├── retention/        Retention cleanup logic
│   └── cleanup.go
├── web/              Web UI (handlers, templates, static)
│   ├── server.go
│   ├── templates/
│   └── static/
├── main.go           Scheduler & orchestration
├── go.mod
├── go.sum
└── README.md
```

---

## 🔧 Environment Variables

### HTTPBACKUP_MAX_PARALLEL

Limits the number of concurrent downloads.

Example:

```bash
HTTPBACKUP_MAX_PARALLEL=10 ./httpbackupgo
```

Default: `5`

---

## 🚫 Ignored Files

The following should not be committed:

- `config.json`
- backup zip files
- temporary files
- logs
- IDE / OS files

See `.gitignore` in the repository.

---

## 🛡 Design Goals

- Offline-first
- Minimal dependencies
- Clear separation of concerns
- Safe concurrency (no race conditions)
- Predictable runtime behavior

---

## 📜 License

MIT License  
© 2026 krootjes
