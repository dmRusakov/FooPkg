package file

import (
	"os"
	"path/filepath"
)

func FolderIsExist(folderPath string) bool {
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		return false
	}
	return true
}

func MakeFolder(basePath, folderName string) error {
	folderPath := filepath.Join(basePath, folderName)
	err := os.MkdirAll(folderPath, 0755)
	if err != nil {
		return err
	}
	return nil
}

func MakeFoldersIfNotExist(basePath string, folderNames []string) error {
	for _, folderName := range folderNames {
		if e := MakeFolderIfNotExist(basePath, folderName); e != nil {
			return e
		}
	}

	return nil
}

func MakeFolderIfNotExist(basePath, folderName string) error {
	folderPath := filepath.Join(basePath, folderName)
	if !FolderIsExist(folderPath) {
		return MakeFolder(basePath, folderName)
	}
	return nil
}

func DeleteFolder(basePath, folderName string) error {
	folderPath := filepath.Join(basePath, folderName)
	err := os.RemoveAll(folderPath)
	if err != nil {
		return err
	}
	return nil
}
