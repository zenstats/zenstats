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

	"github.com/zenstats/zenstats/config"
)

const (
	maxMindDownloadURL = "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=%s&suffix=tar.gz"
	httpTimeout        = 60 * time.Second
	maxDatabaseBytes   = 256 << 20
)

var fallbackDownloadURLs = []string{
	"https://raw.githubusercontent.com/Loyalsoldier/geoip/release/Country-without-asn.mmdb",
	"https://cdn.jsdelivr.net/gh/Loyalsoldier/geoip@release/Country-without-asn.mmdb",
}

func (g *GeoIP) UpdateGeoIPDB(path string) error {
	g.updateMu.Lock()
	defer g.updateMu.Unlock()
	if path == "" {
		var err error
		path, err = g.DownloadGeoIPDB()
		if err != nil {
			return err
		}
		defer os.Remove(path)
	}
	// Stage, validate and atomically rename a NEW inode. Never truncate the file
	// behind a live mmap, and leave the current reader/cache intact on failure.
	target := g.geoDBPath
	if target == "" {
		target = filepath.Join(config.Conf.DataPath, "GeoLite2-City.mmdb")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()
	staged, err := os.CreateTemp(filepath.Dir(target), ".geoip-install-*.mmdb")
	if err != nil {
		return err
	}
	stagedPath := staged.Name()
	defer os.Remove(stagedPath)
	if err = copyBounded(staged, input); err != nil {
		staged.Close()
		return err
	}
	if err = staged.Sync(); err != nil {
		staged.Close()
		return err
	}
	if err = staged.Close(); err != nil {
		return err
	}
	next, err := openDatabase(stagedPath)
	if err != nil {
		return fmt.Errorf("invalid GeoIP update: %w", err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	// A Country-only update can refresh country misses without discarding an
	// existing City DB. City information is never fabricated from a country.
	secondary := g.geoDB != nil && cityDatabase(g.geoDB.Metadata().DatabaseType) && !cityDatabase(next.Metadata().DatabaseType)
	if secondary {
		target = filepath.Join(filepath.Dir(target), "GeoLite2-City-fallback.mmdb")
	}
	if err = os.Rename(stagedPath, target); err != nil {
		next.Close()
		return err
	}
	if secondary {
		previous := g.countryDB
		g.countryDB = next
		if previous != nil {
			_ = previous.Close()
		}
	} else {
		previous := g.geoDB
		g.geoDB = next
		g.geoDBPath = target
		if previous != nil {
			_ = previous.Close()
		}
	}
	if g.cache != nil {
		g.cache.Purge()
	}
	slog.Info("GeoIP database updated", "type", next.Metadata().DatabaseType, "country_fallback", secondary, "built_at", time.Unix(int64(next.Metadata().BuildEpoch), 0))
	return nil
}

// Downloads always return a temporary candidate; only UpdateGeoIPDB installs it.
func (g *GeoIP) DownloadGeoIPDB() (string, error) {
	if config.Conf.MaxmindLicenseKey != "" {
		path, err := g.downloadFromMaxMind()
		if err == nil {
			return path, nil
		}
		slog.Warn("MaxMind download failed; trying country-level fallback", "error", err)
	} else {
		slog.Warn("MaxMind license key not configured; fallback provides country data only")
	}
	return g.downloadFromFallback()
}
func (g *GeoIP) downloadFromMaxMind() (string, error) {
	client := &http.Client{Timeout: httpTimeout}
	res, err := client.Get(fmt.Sprintf(maxMindDownloadURL, config.Conf.MaxmindLicenseKey))
	// net/http errors include the URL and license key. Do not log or wrap them.
	if err != nil {
		return "", fmt.Errorf("MaxMind request failed")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("MaxMind returned HTTP %d", res.StatusCode)
	}
	archive, err := candidateFile()
	if err != nil {
		return "", err
	}
	defer os.Remove(archive.Name())
	if err = copyBounded(archive, res.Body); err != nil {
		archive.Close()
		return "", err
	}
	if err = archive.Close(); err != nil {
		return "", err
	}
	return g.extractGzFile(archive.Name())
}
func (g *GeoIP) downloadFromFallback() (string, error) {
	client := &http.Client{Timeout: httpTimeout}
	var lastErr error
	for _, source := range fallbackDownloadURLs {
		url := config.Conf.GeoIPMirror + source
		res, err := client.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("country database request failed")
			continue
		}
		if res.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("country database returned HTTP %d", res.StatusCode)
			res.Body.Close()
			continue
		}
		out, err := candidateFile()
		if err != nil {
			res.Body.Close()
			return "", err
		}
		err = copyBounded(out, res.Body)
		res.Body.Close()
		closeErr := out.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			var dbErr error
			db, openErr := openDatabase(out.Name())
			dbErr = openErr
			if db != nil {
				db.Close()
			}
			err = dbErr
		}
		if err == nil {
			return out.Name(), nil
		}
		os.Remove(out.Name())
		lastErr = err
	}
	return "", fmt.Errorf("all country GeoIP sources failed: %v", lastErr)
}
func candidateFile() (*os.File, error) {
	if err := os.MkdirAll(config.Conf.DataPath, 0755); err != nil {
		return nil, err
	}
	return os.CreateTemp(config.Conf.DataPath, ".geoip-download-*.mmdb")
}
func copyBounded(dst io.Writer, src io.Reader) error {
	n, err := io.Copy(dst, io.LimitReader(src, maxDatabaseBytes+1))
	if err != nil {
		return err
	}
	if n > maxDatabaseBytes {
		return fmt.Errorf("GeoIP file exceeds %d MiB", maxDatabaseBytes>>20)
	}
	return nil
}
func (g *GeoIP) extractGzFile(path string) (string, error) {
	input, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer input.Close()
	gz, err := gzip.NewReader(input)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return "", fmt.Errorf("GeoIP archive contains no MMDB")
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag != tar.TypeReg || !strings.HasSuffix(header.Name, ".mmdb") {
			continue
		}
		if header.Size > maxDatabaseBytes {
			return "", fmt.Errorf("GeoIP archive entry is too large")
		}
		out, err := candidateFile()
		if err != nil {
			return "", err
		}
		err = copyBounded(out, tr)
		closeErr := out.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(out.Name())
			return "", err
		}
		return out.Name(), nil
	}
}
