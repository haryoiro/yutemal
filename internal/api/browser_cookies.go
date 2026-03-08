package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1" //nolint:gosec // Chrome uses SHA1 for PBKDF2
	"database/sql"
	"fmt"
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
	BrowserChrome BrowserCookieSource = "chrome"
)

// ReadBrowserCookies reads and decrypts YouTube cookies from Chrome on macOS.
func ReadBrowserCookies(browser BrowserCookieSource) (string, error) {
	switch browser {
	case BrowserChrome:
		return readChromeCookies()
	default:
		return "", fmt.Errorf("unsupported browser: %s", browser)
	}
}

func readChromeCookies() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Chrome cookie DB path on macOS
	dbPath := filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default", "Cookies")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return "", fmt.Errorf("Chrome cookie database not found at %s", dbPath)
	}

	// Get encryption key from macOS Keychain
	encKey, err := getChromeEncryptionKey()
	if err != nil {
		return "", fmt.Errorf("failed to get Chrome encryption key: %w", err)
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

	// Query YouTube cookies
	rows, err := db.Query(
		`SELECT name, encrypted_value, value FROM cookies WHERE host_key LIKE '%youtube.com' OR host_key LIKE '%google.com'`,
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
		return "", fmt.Errorf("no YouTube/Google cookies found in Chrome")
	}

	// Check for essential cookies
	if _, ok := cookies["SAPISID"]; !ok {
		return "", fmt.Errorf("SAPISID cookie not found — please log in to YouTube Music in Chrome")
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

// getChromeEncryptionKey retrieves Chrome's encryption key from macOS Keychain.
func getChromeEncryptionKey() ([]byte, error) {
	cmd := exec.Command("security", "find-generic-password", "-w", "-s", "Chrome Safe Storage")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get Chrome Safe Storage key from Keychain: %w", err)
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

	// IV is 16 space bytes (0x20)
	iv := make([]byte, aes.BlockSize)
	for i := range iv {
		iv[i] = ' '
	}

	if len(encrypted)%aes.BlockSize != 0 {
		return "", fmt.Errorf("encrypted data is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encrypted))
	mode.CryptBlocks(decrypted, encrypted)

	// Remove PKCS#7 padding
	if len(decrypted) == 0 {
		return "", nil
	}

	paddingLen := int(decrypted[len(decrypted)-1])
	if paddingLen > aes.BlockSize || paddingLen > len(decrypted) {
		return "", fmt.Errorf("invalid PKCS#7 padding")
	}

	return string(decrypted[:len(decrypted)-paddingLen]), nil
}

// copyToTemp copies a file to a temporary location.
func copyToTemp(src string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp("", "yutemal-cookies-*.db")
	if err != nil {
		return "", err
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}

	tmpFile.Close()

	return tmpFile.Name(), nil
}
