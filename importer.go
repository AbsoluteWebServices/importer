package main

import (
        "bufio"
        "fmt"
        "io"
        "os"
        "os/exec"
        "path/filepath"
        "strconv"
        "strings"
        "time"
        "runtime"
        "encoding/json"

        "github.com/cheggaaa/pb/v3"
        "github.com/pkg/sftp"
        "golang.org/x/crypto/ssh"
)

const (
        RED     = "\033[31m"
        GREEN   = "\033[32m"
        BOLD    = "\033[1m"
        REGULAR = "\033[0m"
        VERSION = "1.4"
        REPOURL = "absolutewebservices/importer"
)

type Config struct {
        Host     string
        Port     string
        User     string
        Password string
        SSHKey   string
        CMSPath  string
}

func main() {
        updateBinary()

        if len(os.Args) >= 2 && (os.Args[1] == "version" || os.Args[1] == "-v" || os.Args[1] == "--version") {
            fmt.Println(BOLD + "Importer version is " + VERSION + REGULAR)
            os.Exit(0)
        }

        if len(os.Args) < 3 {
                printUsage()
                os.Exit(1)
        }

        config := Config{
                Host: os.Args[2],
                Port: "22", // default
        }
        if len(os.Args) > 3 {
                config.Port = os.Args[3]
        }

        switch os.Args[1] {
        case "media":
                runMediaImport(&config)
        case "sql":
                runSQLImport(&config)
        case "both":
                runBothImport(&config)
        default:
                printUsage()
                os.Exit(1)
        }
}


func getLatestReleaseVersion() (string, error) {
        repoURL := "https://api.github.com/repos/" + REPOURL + "/releases/latest"
        cmd := exec.Command("curl", "-s", repoURL)
        output, err := cmd.CombinedOutput()
        if err != nil {
                return "", err
        }
        var data map[string]interface{}
        err = json.Unmarshal(output, &data)
        if err != nil {
                return "", err
        }
        return data["tag_name"].(string), nil
}

func updateBinary() {
        latestVersion, err := getLatestReleaseVersion()
        if err != nil {
                fmt.Printf("%sFailed to get latest version: %v%s\n", RED, err, REGULAR)
                return
        }

        if compareVersions(VERSION, latestVersion) < 0 {
                fmt.Printf("A new version (%s) is available. Do you want to update? (y/n): ", latestVersion)
                reader := bufio.NewReader(os.Stdin)
                input, _ := reader.ReadString('\n')
                input = strings.TrimSpace(input)

                if strings.ToLower(input) != "y" {
                        fmt.Println("Update cancelled.")
                        return
                }
        } else {
                fmt.Println("You are using the latest version.")
                return
        }

        fmt.Println("Updating...")

        binaryName := "importer"

        switch runtime.GOOS {
        case "darwin":
                switch runtime.GOARCH {
                case "amd64":
                        binaryName = "importer-darwin-amd64"
                case "arm64":
                        binaryName = "importer-darwin-arm64"
                default:
                        fmt.Printf("%sUnsupported macOS architecture: %s%s\n", RED, runtime.GOARCH, REGULAR)
                        return
                }
        case "linux":
                switch runtime.GOARCH {
                case "amd64":
                        binaryName = "importer-linux-amd64"
                default:
                        fmt.Printf("%sUnsupported Linux architecture: %s%s\n", RED, runtime.GOARCH, REGULAR)
                        return
                }
        case "windows":
                fmt.Println("Windows update is not supported")
                return
        default:
                fmt.Printf("%sUnsupported OS: %s%s\n", RED, runtime.GOOS, REGULAR)
                return
        }

        tempDir, err := os.MkdirTemp("", "importer-update")
        if err != nil {
                fmt.Printf("%sFailed to create temp dir: %v%s\n", RED, err, REGULAR)
                return
        }
        defer os.RemoveAll(tempDir)

        tempBinaryPath := filepath.Join(tempDir, binaryName)

        downloadCmd := exec.Command("curl", "-L", fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", REPOURL, latestVersion, binaryName), "-o", tempBinaryPath)
        if err := downloadCmd.Run(); err != nil {
                fmt.Printf("%sFailed to download update: %v%s\n", RED, err, REGULAR)
                return
        }

        if err := os.Chmod(tempBinaryPath, 0755); err != nil {
                fmt.Printf("%sFailed to set permissions: %v%s\n", RED, err, REGULAR)
                return
        }

        currentBinaryPath, err := os.Executable()
        if err != nil {
                fmt.Printf("%sFailed to get current binary path: %v%s\n", RED, err, REGULAR)
                return
        }

        if err := os.Rename(tempBinaryPath, currentBinaryPath); err != nil {
                fmt.Printf("%sFailed to replace binary: %v%s\n", RED, err, REGULAR)
                return
        }

        fmt.Printf("\n%sBinary updated successfully!%s\n", GREEN, REGULAR)
        os.Exit(0)
}

func compareVersions(v1, v2 string) int {
        v1 = strings.TrimPrefix(v1, "v")
        v2 = strings.TrimPrefix(v2, "v")

        v1Parts := strings.Split(v1, ".")
        v2Parts := strings.Split(v2, ".")

        for i := 0; i < len(v1Parts) || i < len(v2Parts); i++ {
                v1Num := 0
                if i < len(v1Parts) {
                        v1Num, _ = strconv.Atoi(v1Parts[i])
                }

                v2Num := 0
                if i < len(v2Parts) {
                        v2Num, _ = strconv.Atoi(v2Parts[i])
                }

                if v1Num < v2Num {
                        return -1
                } else if v1Num > v2Num {
                        return 1
                }
        }
        return 0
}


func getUserInput(prompt string) string {
        fmt.Print(prompt)
        scanner := bufio.NewScanner(os.Stdin)
        scanner.Scan()
        return scanner.Text()
}

func setupSSHClient(config *Config) (*ssh.Client, error) {
        config.User = getUserInput("Enter ssh login: ")
        config.Password = getUserInput("Enter ssh password or leave empty: ")

        var authMethod ssh.AuthMethod
        if config.Password != "" {
                authMethod = ssh.Password(config.Password)
        } else {
                keyPath := filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
                key, err := os.ReadFile(keyPath)
                if err != nil {
                        fmt.Printf("%sError reading SSH key: %v%s\n", RED, err, REGULAR)
                        os.Exit(1)
                }
                signer, err := ssh.ParsePrivateKey(key)
                if err != nil {
                        fmt.Printf("%sError parsing SSH key: %v%s\n", RED, err, REGULAR)
                        os.Exit(1)
                }
                authMethod = ssh.PublicKeys(signer)
        }

        sshConfig := &ssh.ClientConfig{
                User:            config.User,
                Auth:            []ssh.AuthMethod{authMethod},
                HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        }

        client, err := ssh.Dial("tcp", config.Host+":"+config.Port, sshConfig)
        if err != nil {
                fmt.Printf("%sSSH connection failed: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        fmt.Printf("%sSSH session open finished successfully!%s\n", GREEN, REGULAR)
        return client, nil
}

func runSQLImport(config *Config) {
        client, err := setupSSHClient(config)
        if err != nil {
                os.Exit(1)
        }
        defer client.Close()

        sftpClient, err := sftp.NewClient(client)
        if err != nil {
                fmt.Printf("%sSFTP client creation failed: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        defer sftpClient.Close()

        config.CMSPath = strings.TrimRight(getUserInput("Enter Magento 1/2 root directory path (NOT PUB). For example: /var/www/website (default: '~/public_html'): "), "/")
        if config.CMSPath == "" || config.CMSPath == "~/public_html" {
                config.CMSPath = fmt.Sprintf("/home/%s/public_html", config.User)
        }

        dbConfig := detectCMSAndGetDBConfig(client, config)
        exportDatabase(client, sftpClient, config, dbConfig)
}

func runMediaImport(config *Config) {
        client, err := setupSSHClient(config)
        if err != nil {
                os.Exit(1)
        }
        defer client.Close()

        sftpClient, err := sftp.NewClient(client)
        if err != nil {
                fmt.Printf("%sSFTP client creation failed: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        defer sftpClient.Close()

        config.CMSPath = strings.TrimRight(getUserInput("Enter Magento 1/2 root directory path (NOT PUB). For example: /var/www/website (default: '~/public_html'): "), "/")
        if config.CMSPath == "" || config.CMSPath == "~/public_html" {
                config.CMSPath = fmt.Sprintf("/home/%s/public_html", config.User)
        }

        mediaPath := detectMediaPath(client, config)
        downloadMedia(client, sftpClient, config, mediaPath)
}

func runBothImport(config *Config) {
        client, err := setupSSHClient(config)
        if err != nil {
                os.Exit(1)
        }
        defer client.Close()

        sftpClient, err := sftp.NewClient(client)
        if err != nil {
                fmt.Printf("%sSFTP client creation failed: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        defer sftpClient.Close()

        config.CMSPath = strings.TrimRight(getUserInput("Enter Magento 1/2 root directory path (NOT PUB). For example: /var/www/website (default: '~/public_html'): "), "/")
        if config.CMSPath == "" || config.CMSPath == "~/public_html" {
                config.CMSPath = fmt.Sprintf("/home/%s/public_html", config.User)
        }

        dbConfig := detectCMSAndGetDBConfig(client, config)
        mediaPath := detectMediaPath(client, config)
        exportDatabase(client, sftpClient, config, dbConfig)
        downloadMedia(client, sftpClient, config, mediaPath)
}

func detectCMSAndGetDBConfig(client *ssh.Client, config *Config) map[string]string {
        session, err := client.NewSession()
        if err != nil {
                fmt.Printf("%sSSH session failed: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        defer session.Close()

        cmd := fmt.Sprintf("[ -f %s/app/etc/local.xml ] && echo 1 || [ -f %s/app/etc/env.php ] && echo 2", config.CMSPath, config.CMSPath)
        output, err := session.CombinedOutput(cmd)
        if err != nil {
                fmt.Printf("%sCouldn't find config file in %s: %v%s\n", RED, config.CMSPath, err, REGULAR)
                os.Exit(1)
        }

        cmsType := strings.TrimSpace(string(output))
        if cmsType == "1" {
                return parseMagento1Config(client, config)
        } else if cmsType == "2" {
                return parseMagento2Config(client, config)
        }
        fmt.Printf("%sUnknown CMS type detected. Expected '1' or '2', got: '%s'%s\n", RED, cmsType, REGULAR)
        os.Exit(1)
        return nil
}

func parseMagento1Config(client *ssh.Client, config *Config) map[string]string {
        session, err := client.NewSession()
        if err != nil {
                fmt.Printf("%sSSH session failed: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        defer session.Close()

        phpCode := fmt.Sprintf(`php -r '$a=file_get_contents("%s/app/etc/local.xml"); $p=xml_parser_create(); xml_parse_into_struct($p, $a, $vals, $index); echo "[dbhost]=".$vals[$index["HOST"][0]]["value"]." [dbname]=".$vals[$index["DBNAME"][0]]["value"]." [dbuser]=".$vals[$index["USERNAME"][0]]["value"]." [dbpass]=".$vals[$index["PASSWORD"][0]]["value"];'`, config.CMSPath)
        output, err := session.CombinedOutput(phpCode)
        if err != nil {
                fmt.Printf("%sFailed to parse Magento 1 config: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        return parseConfigOutput(string(output))
}

func parseMagento2Config(client *ssh.Client, config *Config) map[string]string {
        session, err := client.NewSession()
        if err != nil {
                fmt.Printf("%sSSH session failed: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        defer session.Close()

        phpCode := fmt.Sprintf(`php -r '$a=require("%s/app/etc/env.php"); echo "[dbhost]=".$a["db"]["connection"]["default"]["host"]." [dbname]=".$a["db"]["connection"]["default"]["dbname"]." [dbuser]=".$a["db"]["connection"]["default"]["username"]." [dbpass]=".$a["db"]["connection"]["default"]["password"];'`, config.CMSPath)
        output, err := session.CombinedOutput(phpCode)
        if err != nil {
                fmt.Printf("%sFailed to parse Magento 2 config: %v - %s%s\n", RED, err, string(output), REGULAR)
                os.Exit(1)
        }
        return parseConfigOutput(string(output))
}

func parseConfigOutput(output string) map[string]string {
        result := make(map[string]string)
        parts := strings.Fields(output)
        for _, part := range parts {
                if strings.Contains(part, "=") {
                        kv := strings.SplitN(part, "=", 2)
                        key := strings.Trim(kv[0], "[]")
                        value := kv[1]
                        result[key] = value
                }
        }
        return result
}

func exportDatabase(client *ssh.Client, sftpClient *sftp.Client, config *Config, dbConfig map[string]string) {
    session, err := client.NewSession()
    if err != nil {
        fmt.Printf("%sSSH session failed: %v%s\n", RED, err, REGULAR)
        os.Exit(1)
    }
    defer session.Close()

    passArg := ""
    if dbConfig["dbpass"] != "" {
        passArg = fmt.Sprintf("-p'%s'", dbConfig["dbpass"])
    }

    // Command to get the database size
    sizeCmd := fmt.Sprintf("mysql -h %s -u%s %s -e \"SELECT ROUND(SUM(data_length + index_length)) AS 'SIZE' FROM information_schema.TABLES WHERE table_schema = '%s';\" | grep -v SIZE",
        dbConfig["dbhost"], dbConfig["dbuser"], passArg, dbConfig["dbname"])
    
    sizeOutput, err := session.CombinedOutput(sizeCmd)
    if err != nil {
        fmt.Printf("%sFailed to get database size: %v (output: %s)%s\n", RED, err, string(sizeOutput), REGULAR)
        os.Exit(1)
    }

    // Split output by lines and take the last non-empty line
    outputStr := strings.TrimSpace(string(sizeOutput))
    lines := strings.Split(outputStr, "\n")
    var sizeStr string
    for i := len(lines) - 1; i >= 0; i-- {
        if strings.TrimSpace(lines[i]) != "" {
            sizeStr = lines[i]
            break
        }
    }

    // Parse the size
    sizeInt, err := strconv.ParseInt(strings.TrimSpace(sizeStr), 10, 64)
    if err != nil {
        fmt.Printf("%sFailed to parse database size: %v (output: %s)%s\n", RED, err, outputStr, REGULAR)
        os.Exit(1)
    }
    sizeAfterDivision := sizeInt / 8

    fmt.Printf("%s!!! Estimated compressed db file size: %s%s\n", BOLD, humanReadableSize(sizeAfterDivision), REGULAR)

    // Rest of the function remains unchanged...
    dumpFile := fmt.Sprintf("auto_%s_%s.sql.gz", dbConfig["dbname"], time.Now().Format("02_01-15_04"))
    dumpCmd := fmt.Sprintf("mysqldump -h %s %s -u%s %s --no-tablespaces --routines --skip-triggers --single-transaction | gzip -9",
        dbConfig["dbhost"], dbConfig["dbname"], dbConfig["dbuser"], passArg)

    session, err = client.NewSession()
    if err != nil {
        fmt.Printf("%sSSH session failed: %v%s\n", RED, err, REGULAR)
        os.Exit(1)
    }
    defer session.Close()

    localFile, err := os.Create(dumpFile)
    if err != nil {
        fmt.Printf("%sFailed to create local file: %v%s\n", RED, err, REGULAR)
        os.Exit(1)
    }
    defer localFile.Close()

    bar := pb.Full.Start64(sizeAfterDivision)
    barReader, barWriter := io.Pipe()
    session.Stdout = barWriter

    go func() {
        defer barWriter.Close()
        if err := session.Run(dumpCmd); err != nil {
            fmt.Printf("%sDatabase export failed: %v%s\n", RED, err, REGULAR)
            os.Exit(1)
        }
    }()

    _, err = io.Copy(io.MultiWriter(localFile, bar.NewProxyWriter(io.Discard)), barReader)
    if err != nil {
        fmt.Printf("%sFailed to write local file: %v%s\n", RED, err, REGULAR)
        os.Exit(1)
    }
    bar.Finish()

    fmt.Printf("%sDatabase export completed%s\n", GREEN, REGULAR)

    if err := exec.Command("gzip", "-d", dumpFile).Run(); err != nil {
        fmt.Printf("%sDecompression failed: %v%s\n", RED, err, REGULAR)
        os.Exit(1)
    }
    fmt.Printf("%sSQL download finished successfully! File: %s%s\n", GREEN, dumpFile[:len(dumpFile)-3], REGULAR)
}

func detectMediaPath(client *ssh.Client, config *Config) string {
        session, err := client.NewSession()
        if err != nil {
                fmt.Printf("%sSSH session failed: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        defer session.Close()

        cmd := fmt.Sprintf("[ -f %s/app/etc/local.xml ] && echo 1 || [ -f %s/app/etc/env.php ] && echo 2", config.CMSPath, config.CMSPath)
        output, err := session.CombinedOutput(cmd)
        if err != nil {
                fmt.Printf("%sCouldn't find config file in %s: %v%s\n", RED, config.CMSPath, err, REGULAR)
                os.Exit(1)
        }

        cmsType := strings.TrimSpace(string(output))
        if cmsType == "1" {
                return config.CMSPath
        } else if cmsType == "2" {
                return filepath.Join(config.CMSPath, "pub")
        }
        fmt.Printf("%sUnknown CMS type detected%s\n", RED, REGULAR)
        os.Exit(1)
        return ""
}

func humanReadableSize(bytes int64) string {
        const (
                KB = 1024
                MB = KB * 1024
                GB = MB * 1024
        )
        switch {
        case bytes >= GB:
                return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
        case bytes >= MB:
                return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
        case bytes >= KB:
                return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
        default:
                return fmt.Sprintf("%d bytes", bytes)
        }
}

func downloadMedia(client *ssh.Client, sftpClient *sftp.Client, config *Config, mediaPath string) {
        // Step 1: Calculate uncompressed size
        session, err := client.NewSession()
        if err != nil {
                fmt.Printf("%sSSH session failed: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        defer session.Close()

        fmt.Println("Calculating media directory size...")
        sizeCmd := fmt.Sprintf("du -sb %s/media/ | awk '{print $1}'", mediaPath)
        sizeOutput, err := session.CombinedOutput(sizeCmd)
        if err != nil {
                fmt.Printf("%sFailed to calculate media size: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        size, err := strconv.ParseInt(strings.TrimSpace(string(sizeOutput)), 10, 64)
        sizeAfterDiscount := float64(size) - (float64(size) * 0.3)

        if err != nil {
                fmt.Printf("%sFailed to parse media size: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        fmt.Printf("Uncompressed media size: %s\n", humanReadableSize(size))
        fmt.Printf("%s!!! Estimated compressed media size: %s%s\n", BOLD, humanReadableSize(int64(sizeAfterDiscount)),REGULAR)

        // Step 2: Stream tar and gzip directly to local file with progress bar
        session, err = client.NewSession()
        if err != nil {
                fmt.Printf("%sSSH session failed: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        defer session.Close()

        tarCmd := fmt.Sprintf("cd %s && tar -czf - --exclude='cache' media/*", mediaPath)
        localFileName := fmt.Sprintf("%s_media.tar.gz", config.User)
        localFile, err := os.Create(localFileName)
        if err != nil {
                fmt.Printf("%sFailed to create local file: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        defer localFile.Close()

        // Set up progress bar (based on uncompressed size)
        bar := pb.Full.Start64(int64(sizeAfterDiscount))
        barReader, barWriter := io.Pipe()
        session.Stdout = barWriter

        // Copy data to local file and update progress bar concurrently
        go func() {
                defer barWriter.Close()
                if err := session.Run(tarCmd); err != nil {
                        fmt.Printf("%sMedia download failed: %v%s\n", RED, err, REGULAR)
                        os.Exit(1)
                }
        }()

        _, err = io.Copy(io.MultiWriter(localFile, bar.NewProxyWriter(io.Discard)), barReader)
        if err != nil {
                fmt.Printf("%sFailed to write local file: %v%s\n", RED, err, REGULAR)
                os.Exit(1)
        }
        bar.Finish()

        fmt.Printf("%sMedia download finished successfully! File: %s%s\n", GREEN, localFileName, REGULAR)
}

func printUsage() {
        fmt.Println("Usage: importer [media|sql|both] [server ip address] [ssh port (default 22)]")
        fmt.Println("media - copying media files in archive to the local node")
        fmt.Println("sql - create sql dump and download to the local node")
        fmt.Println("both - download media and sql dump to the local node")
}

