# Importer

**Importer** is a lightweight command-line tool written in Go that allows you to securely import media files and SQL dumps from a remote server via SSH.

---

## 📦 Releases

Platform-specific binaries are available in the [Releases](https://github.com/AbsoluteWebServices/importer/releases) section.

| Filename                  | OS     | Architecture   | Download |
|---------------------------|--------|----------------|----------|
| `importer-darwin-amd64`   | macOS  | Intel (x86_64) | [⬇️ Download](https://github.com/AbsoluteWebServices/importer/releases/latest/download/importer-darwin-amd64) |
| `importer-darwin-arm64`   | macOS  | Apple Silicon  | [⬇️ Download](https://github.com/AbsoluteWebServices/importer/releases/latest/download/importer-darwin-arm64) |
| `importer-linux-amd64`    | Linux  | Intel/AMD      | [⬇️ Download](https://github.com/AbsoluteWebServices/importer/releases/latest/download/importer-linux-amd64) |



## 🔧 Usage

Run the tool using the following syntax:

```
./importer [media|sql|both] [server IP address] [SSH port (optional, default: 22)]
```
### Arguments
`media` – Copies media files (in an archive format) from the remote server to the local machine.

`sql` – Creates a SQL dump on the remote server and downloads it to the local machine.

`both` – Downloads both media files and a SQL dump.

### Example
```
./importer both 192.168.1.100
```
This will use SSH to connect to 192.168.1.100 (on port 22) and download both media and SQL files.

## 🔐 SSH Authentication
You can authenticate using:

`Password` – You'll be prompted to enter the password when connecting.

`SSH key` – If no password is provided, the app will try to use your default key at `~/.ssh/id_rsa`.

## ⬇️ How to Download and Use
### 1. Download the right version
Go to the Releases page and download the correct binary for your system.

### 2. Make it executable
After downloading, run the following command in your terminal:

```
chmod +x importer-<your-platform>
```
Replace `<your-platform>` with the name of the file you downloaded (e.g., importer-darwin-arm64).

### 3. Run it
Use ./importer-<your-platform> from the terminal, or optionally rename it:

```
mv importer-<your-platform> importer

./importer media 192.168.1.100
```
## ❓ Common Issues
- **"Permission denied" error** – Make sure the file is executable by running:
```
chmod +x importer-<your-platform>
```

- **"Unidentified developer"** warning – macOS may block the app. To allow it:
1. Go to System Preferences → Security & Privacy → General.
2. Click "Allow Anyway" after attempting to run the binary.

- **"Architecture mismatch"** – Apple Silicon users should download the `arm64` version. Intel Macs need the `amd64` version.

## 📄 License

This project is licensed under the [MIT License](./LICENSE).  
You are free to use, modify, and distribute it with attribution.

