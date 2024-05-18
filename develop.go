//go:build Develop

package main

const port = 8080

func getDbPath() string {
	return "db.sqlite"
}
