# File Uploader — Installation Guide

## Quick Start (Systemd)

1. Extract the distribution archive.
2. Edit `file-uploader.toml` with your configuration.
3. Run the install script as root:

```bash
sudo ./install-systemd.sh
```

4. Start the service:

```bash
sudo systemctl start file-uploader
sudo systemctl enable file-uploader
```

## Quick Start (Docker)

1. Build the Docker image (or use a pre-built one):

```bash
docker build -t file-uploader .
```

2. Run:

```bash
docker run -d -p 8080:8080 -v /path/to/data:/data file-uploader
```

## Configuration

Edit `file-uploader.toml` before installation. Key settings:

- `api-endpoint` — API endpoint in `environment,url` format
- `address` — Listen address (default: `0.0.0.0`)
- `port` — Listen port (default: `8080`)
- `signing-key` — JWT signing key for session tokens

Generate a default config:

```bash
./file-uploader gen-config > file-uploader.toml
```

## Directory Structure (Systemd)

After installation, files are located at:

```
/opt/file-uploader/
  file-uploader          # binary
  file-uploader.toml     # configuration
  data/
    players-db/work/     # player deduplication database
    data-processing/
      upload/            # incoming CSV files
      processing/        # files being processed
      uploading/         # files being uploaded to API
      archive/           # completed files
```

## Logs

```bash
journalctl -u file-uploader -f
```
