package api

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1" //nolint:gosec // Chrome uses SHA1 for PBKDF2
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/pbkdf2"
)

// BrowserCookieSource represents a browser to read cookies from.
type BrowserCookieSource string

const (
	BrowserChrome       BrowserCookieSource = "chrome"
	BrowserChromeBeta   BrowserCookieSource = "chrome-beta"
	BrowserChromeCanary BrowserCookieSource = "chrome-canary"
	BrowserChromium     BrowserCookieSource = "chromium"
)

// browserConfig holds browser-specific paths and keychain info.
type browserConfig struct {
	appSupportDir string // relative to ~/Library/Application Support/
	keychainName  string // Keychain service name
	ytdlpName     string // yt-dlp --cookies-from-browser value
}

var browserConfigs = map[BrowserCookieSource]browserConfig{
	BrowserChrome: {
		appSupportDir: "Google/Chrome",
		keychainName:  "Chrome Safe Storage",
		ytdlpName:     "chrome",
	},
	BrowserChromeBeta: {
		appSupportDir: "Google/Chrome Beta",
		keychainName:  "Chrome Safe Storage",
		ytdlpName:     "chrome",
	},
	BrowserChromeCanary: {
		appSupportDir: "Google/Chrome Canary",
		keychainName:  "Chrome Safe Storage",
		ytdlpName:     "chrome",
	},
	BrowserChromium: {
		appSupportDir: "Chromium",
		keychainName:  "Chromium Safe Storage",
		ytdlpName:     "chromium",
	},
}

// ParseBrowser parses a browser name string into a BrowserCookieSource.
func ParseBrowser(name string) (BrowserCookieSource, bool) {
	switch BrowserCookieSource(name) {
	case BrowserChrome, BrowserChromeBeta, BrowserChromeCanary, BrowserChromium:
		return BrowserCookieSource(name), true
	default:
		return "", false
	}
}

// YtdlpBrowserArg returns the argument for yt-dlp --cookies-from-browser.
// Format: "browser:profile" or just "browser" if profile is empty.
func YtdlpBrowserArg(browser BrowserCookieSource, profile string) string {
	cfg, ok := browserConfigs[browser]
	if !ok {
		return string(browser)
	}
	if profile != "" {
		return cfg.ytdlpName + ":" + profile
	}
	return cfg.ytdlpName
}

// ReadBrowserCookies reads and decrypts YouTube cookies from a Chromium-based browser on macOS.
func ReadBrowserCookies(browser BrowserCookieSource) (string, error) {
	return ReadBrowserCookiesWithProfile(browser, "Default")
}

// ReadBrowserCookiesWithProfile reads cookies from a specific browser profile.
func ReadBrowserCookiesWithProfile(browser BrowserCookieSource, profile string) (string, error) {
	cfg, ok := browserConfigs[browser]
	if !ok {
		return "", fmt.Errorf("unsupported browser: %s", browser)
	}
	if profile == "" {
		profile = "Default"
	}
	return readChromiumCookies(cfg, profile)
}

func readChromiumCookies(cfg browserConfig, profile string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Cookie DB path on macOS
	dbPath := filepath.Join(home, "Library", "Application Support", cfg.appSupportDir, profile, "Cookies")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return "", fmt.Errorf("cookie database not found at %s", dbPath)
	}

	// Get encryption key from macOS Keychain
	encKey, err := getKeychainPassword(cfg.keychainName)
	if err != nil {
		return "", fmt.Errorf("failed to get encryption key: %w", err)
	}

	// Derive AES key using PBKDF2
	aesKey := pbkdf2.Key(encKey, []byte("saltysalt"), 1003, 16, sha1.New)

	// Copy the DB to a temp file to avoid locking issues with Chrome
	tmpDB, err := copyToTemp(dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to copy cookie database: %w", err)
	}
	defer os.Remove(tmpDB)

	// Open SQLite database
	db, err := sql.Open("sqlite3", tmpDB+"?mode=ro")
	if err != nil {
		return "", fmt.Errorf("failed to open cookie database: %w", err)
	}
	defer db.Close()

	// Query YouTube cookies (only .youtube.com domain, matching what browser sends)
	rows, err := db.Query(
		`SELECT name, encrypted_value, value FROM cookies WHERE host_key = '.youtube.com'`,
	)
	if err != nil {
		return "", fmt.Errorf("failed to query cookies: %w", err)
	}
	defer rows.Close()

	cookies := make(map[string]string)

	for rows.Next() {
		var name string
		var encryptedValue []byte
		var plainValue string

		if err := rows.Scan(&name, &encryptedValue, &plainValue); err != nil {
			continue
		}

		// Use plain value if available
		if plainValue != "" {
			cookies[name] = plainValue
			continue
		}

		// Decrypt encrypted value
		if len(encryptedValue) > 3 && string(encryptedValue[:3]) == "v10" {
			decrypted, err := decryptChromeValue(encryptedValue[3:], aesKey)
			if err != nil {
				continue
			}
			cookies[name] = decrypted
		}
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error iterating cookies: %w", err)
	}

	if len(cookies) == 0 {
		return "", fmt.Errorf("no YouTube/Google cookies found")
	}

	// Check for essential cookies
	if _, ok := cookies["SAPISID"]; !ok {
		return "", fmt.Errorf("SAPISID cookie not found — please log in to YouTube Music in the browser")
	}

	// Build cookie header string (only YouTube-essential cookies to avoid header size issues)
	essentialNames := map[string]bool{
		"SAPISID": true, "__Secure-1PAPISID": true, "__Secure-3PAPISID": true,
		"SID": true, "__Secure-1PSID": true, "__Secure-3PSID": true,
		"HSID": true, "SSID": true, "APISID": true,
		"SIDCC": true, "__Secure-1PSIDCC": true, "__Secure-3PSIDCC": true,
		"__Secure-1PSIDTS": true, "__Secure-3PSIDTS": true,
		"LOGIN_INFO": true, "PREF": true, "YSC": true,
		"VISITOR_INFO1_LIVE": true, "VISITOR_PRIVACY_METADATA": true,
		"__Secure-ROLLOUT_TOKEN": true, "__Host-1PLSID": true, "__Host-3PLSID": true,
	}

	var parts []string
	for name, value := range cookies {
		if !essentialNames[name] {
			continue
		}
		// Sanitize value: remove control characters
		sanitized := strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f {
				return -1 // drop
			}
			return r
		}, value)
		parts = append(parts, fmt.Sprintf("%s=%s", name, sanitized))
	}

	return strings.Join(parts, "; "), nil
}

// getKeychainPassword retrieves a browser's encryption key from macOS Keychain.
func getKeychainPassword(serviceName string) ([]byte, error) {
	cmd := exec.Command("security", "find-generic-password", "-w", "-s", serviceName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get %q key from Keychain: %w", serviceName, err)
	}

	return []byte(strings.TrimSpace(string(output))), nil
}

// decryptChromeValue decrypts a Chrome cookie value using AES-128-CBC.
func decryptChromeValue(encrypted, key []byte) (string, error) {
	if len(encrypted) == 0 {
		return "", nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(encrypted)%aes.BlockSize != 0 {
		return "", fmt.Errorf("encrypted data is not a multiple of block size")
	}

	// Decrypt using AES-128-CBC with space IV (standard Chrome macOS format).
	iv := bytes.Repeat([]byte{' '}, aes.BlockSize)
	decrypted := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(decrypted, encrypted)

	result, err := removePKCS7Padding(decrypted)
	if err != nil {
		return "", err
	}

	// Newer Chrome versions prepend a 32-byte nonce to the plaintext before encryption.
	// After decryption, skip the nonce to get the actual cookie value.
	// Try with 32-byte skip first; fall back to no skip for older Chrome versions.
	const nonceLen = 32
	if len(result) > nonceLen && isAllPrintableASCII(result[nonceLen:]) {
		return string(result[nonceLen:]), nil
	}
	if isAllPrintableASCII(result) {
		return string(result), nil
	}

	return "", fmt.Errorf("failed to decrypt cookie value")
}

// removePKCS7Padding removes PKCS#7 padding from decrypted data.
func removePKCS7Padding(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	paddingLen := int(data[len(data)-1])
	if paddingLen == 0 || paddingLen > aes.BlockSize || paddingLen > len(data) {
		return nil, fmt.Errorf("invalid PKCS#7 padding")
	}
	// Verify all padding bytes
	for i := len(data) - paddingLen; i < len(data); i++ {
		if data[i] != byte(paddingLen) {
			return nil, fmt.Errorf("invalid PKCS#7 padding byte")
		}
	}
	return data[:len(data)-paddingLen], nil
}

// isAllPrintableASCII checks if all bytes are printable ASCII.
func isAllPrintableASCII(data []byte) bool {
	for _, b := range data {
		if b < 0x20 || b > 0x7e {
			return false
		}
	}
	return len(data) > 0
}

// copyToTemp copies a file to a temporary location using streaming io.Copy
// to avoid loading the entire file into memory.
func copyToTemp(src string) (string, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	tmpFile, err := os.CreateTemp("", "yutemal-cookies-*.db")
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}

	tmpFile.Close()

	return tmpFile.Name(), nil
}
