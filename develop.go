//go:build Develop

package main

func getSecret() (string, error) {
	return "test", nil
}

func getDbPath() string {
	return "db.sqlite"
}
