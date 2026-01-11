# httpBackupGo

httpBackupGo is a Go application designed to automate the backup of websites. It allows users to configure multiple sites, set backup intervals, and manage retention policies for backups.

## Features

- **Concurrent Backups**: Supports downloading backups from multiple sites concurrently.
- **Configurable Settings**: Users can specify backup intervals, retention periods, and site details through a configuration file.
- **Web Interface**: A simple web UI to manage configurations and trigger backups.

## Project Structure

```
httpBackupGo/
├── .gitattributes
├── .gitignore
├── go.mod
├── main.go
├── README.md
├── backup/
│   └── runner.go
├── config/
│   └── config.go
├── retention/
│   └── cleanup.go
└── web/
    ├── server.go
    ├── static/
    │   ├── bootstrap.bundle.min.js
    │   └── bootstrap.min.css
    └── templates/
        └── index.html
```

## Installation

1. Clone the repository:
   ```sh
   git clone https://github.com/yourusername/httpBackupGo.git
   cd httpBackupGo
   ```

2. Build the application:
   ```sh
   go build -o httpBackupGo
   ```

3. Run the application:
   ```sh
   ./httpBackupGo
   ```

## Configuration

The application uses a JSON configuration file to manage settings. The default configuration file is located at:

- **Windows**: `%ProgramData%\httpBackupGo\config.json`
- **Linux**: `config.json`

You can modify the configuration file to set the following parameters:

- `IntervalMinutes`: The interval in minutes for scheduled backups.
- `BackupFolder`: The folder where backups will be stored.
- `Retention`: The number of days to keep backups.
- `Sites`: A list of sites to back up, each with an `Enabled`, `Name`, and `Url`.

### Example Configuration

```json
{
  "IntervalMinutes": 5,
  "BackupFolder": "httpBackupGo",
  "Retention": 30,
  "Sites": [
    {
      "Enabled": true,
      "Name": "example",
      "Url": "http://localhost:81/backup.zip"
    }
  ]
}
```

## Web Interface

After starting the application, you can access the web interface at `http://127.0.0.1:8080`. Here, you can manage your backup configurations and trigger backups manually.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.