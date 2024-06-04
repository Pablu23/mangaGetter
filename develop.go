//go:build Develop

package main

func getSecretPath() (string, error) {
	return "", nil
}

func getSecret() (string, error) {
	return "test", nil
}

func getDbPath() string {
	return "db.sqlite"
}
