//go:build !Develop

package main

import (
	"os"
	"path/filepath"
)

func getSecret() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	dirPath := filepath.Join(dir, "MangaGetter")
	filePath := filepath.Join(dirPath, "secret.secret")
	buf, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

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
