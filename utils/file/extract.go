package file

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dsnet/compress/bzip2"
)

// ExtractBz2File extracts a file (bz2)
func ExtractBz2File(filePath, directionsPath string) (string, error) {
	// Open the bz2 file
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Create bz2 reader
	bz2Reader, err := bzip2.NewReader(file, nil)
	if err != nil {
		return "", err
	}

	// Determine output file name: remove .bz2 extension
	base := filepath.Base(filePath)
	if strings.HasSuffix(base, ".bz2") {
		base = base[:len(base)-4]
	}
	outputPath := filepath.Join(directionsPath, base)

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	// Copy from bz2 reader to output file
	_, err = io.Copy(outFile, bz2Reader)
	if err != nil {
		return "", err
	}

	return outputPath, nil
}

// ExtractZipFile extracts a zip archive to dest.
func ExtractZipFile(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// prevent zip-slip
		if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
