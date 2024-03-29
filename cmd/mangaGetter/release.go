//go:build !Develop

package main

import (
	"os"
	"path/filepath"
)

func getDbPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		panic(err)
	}

	dirPath := filepath.Join(dir, "MangaGetter")
	filePath := filepath.Join(dirPath, "db.sqlite")

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		err = os.Mkdir(dirPath, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		f, err := os.Create(filePath)
		if err != nil {
			panic(err)
		}
		err = f.Close()
		if err != nil {
			panic(err)
		}
	}

	return filePath
}
