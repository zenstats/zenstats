package geoip

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dromara/carbon/v2"
	"github.com/oschwald/geoip2-golang"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/pkg/file"
)

const (
	maxMindDownloadURL = "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=%s&suffix=tar.gz"
	httpTimeout        = 60 * time.Second
)

var fallbackDownloadURLs = []string{
	"https://github.com/Loyalsoldier/geoip/raw/release/Country-without-asn.mmdb",
	"https://cdn.jsdelivr.net/gh/Loyalsoldier/geoip@release/Country-without-asn.mmdb",
}

func (g *GeoIP) UpdateGeoIPDB(path string) error {
	// Download the new GeoIP database
	if path == "" {
		newDBPath, err := g.DownloadGeoIPDB()
		if err != nil {
			return fmt.Errorf("download geoip file failed with error: %v", err)
		}
		path = newDBPath
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	// Generate a random backup name for the old database
	backupName := "geoip_backup.mmdb"
	backupPath := filepath.Join(filepath.Dir(g.geoDBPath), backupName)

	// Delete old backup file
	if _, err := os.Stat(backupPath); err == nil {
		if err := os.Remove(backupPath); err != nil {
			return fmt.Errorf("failed to remove gz file: %v", err)
		}
	}

	// Rename the old database to the backup name
	if !strings.HasSuffix(g.geoDBPath, "GeoLite2-City.mmdb") {
		if err := os.Rename(g.geoDBPath, backupPath); err != nil {
			return err
		}
	}

	// Open the new GeoIP database
	newDB, err := geoip2.Open(path)
	if err != nil {
		return err
	}

	// Close the current GeoIP database
	if err := g.geoDB.Close(); err != nil {
		return err
	}
	// Replace the old GeoIP database with the new one
	g.geoDB = newDB
	g.geoDBPath = path

	slog.Info(fmt.Sprintf("GeoIP database updated to %s. Old database backed up to %s", path, backupPath))
	// Copy the new database to default file
	file.CopyFile(g.geoDBPath, filepath.Join(config.Conf.DataPath, "GeoLite2-City.mmdb"))

	return nil
}

func (g *GeoIP) DownloadGeoIPDB() (string, error) {
	if config.Conf.MaxmindLicenseKey != "" {
		path, err := g.downloadFromMaxMind()
		if err == nil {
			return path, nil
		}
		slog.Warn("MaxMind download failed, falling back to Loyalsoldier/geoip", "error", err)
	} else {
		slog.Warn("MaxMind license key not configured, using Loyalsoldier/geoip as fallback")
	}

	return g.downloadFromFallback()
}

func (g *GeoIP) downloadFromMaxMind() (string, error) {
	url := fmt.Sprintf(maxMindDownloadURL, config.Conf.MaxmindLicenseKey)

	if _, err := os.Stat(config.Conf.DataPath); os.IsNotExist(err) {
		if err := os.MkdirAll(config.Conf.DataPath, os.ModePerm); err != nil {
			return "", fmt.Errorf("failed to create directory: %v", err)
		}
	}

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download from MaxMind: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("MaxMind download returned status %d", resp.StatusCode)
	}

	outputPath := filepath.Join(config.Conf.DataPath, "GeoLite2-City.mmdb.tar.gz")

	outFile, err := os.Create(outputPath)
	if err != nil {
		slog.Error("failed to create file: " + err.Error())
		return "", err
	}
	if _, err := io.Copy(outFile, resp.Body); err != nil {
		slog.Error("failed to write file: " + err.Error())
		return "", err
	}
	outFile.Close()

	mmdbPath, err := g.extractGzFile(outputPath)
	if err != nil {
		slog.Error("failed to extract gz file: " + err.Error())
		return "", err
	}

	return mmdbPath, nil
}

func (g *GeoIP) downloadFromFallback() (string, error) {
	if _, err := os.Stat(config.Conf.DataPath); os.IsNotExist(err) {
		if err := os.MkdirAll(config.Conf.DataPath, os.ModePerm); err != nil {
			return "", fmt.Errorf("failed to create directory: %v", err)
		}
	}

	client := &http.Client{Timeout: httpTimeout}

	var lastErr error
	for _, url := range fallbackDownloadURLs {
		slog.Info("Trying fallback GeoIP source", "url", url)
		resp, err := client.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("failed to download from %s: %v", url, err)
			slog.Warn("Fallback download failed", "url", url, "error", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("fallback download returned status %d from %s", resp.StatusCode, url)
			slog.Warn("Fallback download returned non-200", "url", url, "status", resp.StatusCode)
			continue
		}

		outputPath := filepath.Join(config.Conf.DataPath, "GeoLite2-City-fallback.mmdb")
		outFile, err := os.Create(outputPath)
		if err != nil {
			return "", fmt.Errorf("failed to create fallback file: %v", err)
		}
		if _, err := io.Copy(outFile, resp.Body); err != nil {
			outFile.Close()
			return "", fmt.Errorf("failed to write fallback file: %v", err)
		}
		outFile.Close()

		slog.Info("Successfully downloaded fallback GeoIP database", "url", url)
		return outputPath, nil
	}

	return "", fmt.Errorf("all fallback sources failed, last error: %v", lastErr)
}

func (g *GeoIP) extractGzFile(gzFilePath string) (string, error) {
	// 判断默认文件是否存在 如果不存在则保存至默认文件
	outPutFile := defaultDB
	if _, err := os.Stat(defaultDB); err == nil {
		outPutFile = filepath.Join(config.Conf.DataPath, fmt.Sprintf("GeoLite2-City-%s.mmdb", carbon.Now().ToDateString()))
		slog.Info("The default file does not exist, save to default file", "geoipFile", outPutFile)
	}

	// 打开 .gz 文件
	file, err := os.Open(gzFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open .gz file: %v", err)
	}

	// 创建 gzip 解压器
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzipReader.Close()

	// 创建 tar 解析器
	tarReader := tar.NewReader(gzipReader)

	// 遍历 tar 文件中的每个文件
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // 结束遍历
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar header: %v", err)
		}
		if strings.HasSuffix(header.Name, ".mmdb") {
			// 创建文件
			file, err := os.OpenFile(outPutFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return "", fmt.Errorf("failed to create file: %v", err)
			}
			defer file.Close()

			// 将内容写入文件
			if _, err := io.Copy(file, tarReader); err != nil {
				return "", fmt.Errorf("failed to write file: %v", err)
			}
		}
	}
	file.Close()
	// 删除gz文件
	if err := os.Remove(gzFilePath); err != nil {
		return "", fmt.Errorf("failed to remove gz file: %v", err)
	}

	return outPutFile, nil
}
