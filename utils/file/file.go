package file

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// HTTPDownloadFile downloads a file from the given URL and saves it to the specified filepath
//
// @param url The URL of the file to download.
// @param filepath The local path where the downloaded file should be saved.
//
// @return An error if the download fails or if there is an issue creating the file, otherwise nil.
func HTTPDownloadFile(rawURL string, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Encode any literal spaces in the URL before sending.
	req, err := http.NewRequest(http.MethodGet, strings.ReplaceAll(rawURL, " ", "%20"), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FooDataParser)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s, status code: %d", rawURL, resp.StatusCode)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

// IsFileExist checks if a file exists at the given filepath.
//
// @param filepath The path to the file.
//
// @return true if the file exists, false otherwise.
func IsFileExist(filepath string) bool {
	_, err := os.Stat(filepath)
	return !os.IsNotExist(err)
}

// IsFilesExist checks if all files in the given list of filepaths exist.
//
// @param filepaths A slice of file paths to check.
//
// @return true if all files exist, false if any file does not exist.
func IsFilesExist(filepaths []string) bool {
	for _, fp := range filepaths {
		if !IsFileExist(fp) {
			return false
		}
	}
	return true
}

func GetFileBytes(filepath string) ([]byte, error) {
	// Open the file
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the file content into a byte slice
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return fileBytes, nil
}

// MoveFile moves a file from source to destination.
// If destination is an existing directory, the file is moved into that directory
// keeping its original name. Otherwise, destination is treated as a full file path.
func MoveFile(source, destination string) error {
	// If destination is an existing directory, append the source filename.
	info, err := os.Stat(destination)
	if err == nil && info.IsDir() {
		destination = filepath.Join(destination, filepath.Base(source))
	}

	return os.Rename(source, destination)
}

func GetFileHash(filepath string) (string, error) {
	// Open the file
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Create a new hash object
	hash := sha256.New()

	// Copy the file content to the hash object
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// Get the final hash value as a hexadecimal string
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func DeleteFile(filepath string) error {
	err := os.Remove(filepath)
	if err != nil {
		return err
	}

	return nil
}
