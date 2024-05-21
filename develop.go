//go:build Develop

package main

const port = 8080

func getSecret() string {
	return "test"
}

func getDbPath() string {
	return "db.sqlite"
}
