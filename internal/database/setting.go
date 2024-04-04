package database

import (
	"database/sql"
)

type Setting struct {
	Name    string
	Value   string
	Default string
}

func NewSetting(name string, defaultValue string) Setting {
	return Setting{
		Name:    name,
		Value:   defaultValue,
		Default: defaultValue,
	}
}

func initSettings(settings *DbTable[string, Setting]) {
	addSettingIfNotExists("theme", "white", settings)
	addSettingIfNotExists("order", "title", settings)
}

func addSettingIfNotExists(name string, value string, settings *DbTable[string, Setting]) {
	_, exists := settings.Get(name)
	if !exists {
		settings.Set(name, NewSetting(name, value))
	}
}

func updateSetting(db *sql.DB, s *Setting) error {
	const cmd = "UPDATE Setting set Value = ? WHERE Name = ?"
	_, err := db.Exec(cmd, s.Value, s.Name)
	return err
}

func insertSetting(db *sql.DB, s *Setting) error {
	const cmd = "INSERT INTO Setting(Name, Value, DefaultValue) VALUES(?, ?, ?)"
	_, err := db.Exec(cmd, s.Name, s.Value, s.Default)
	return err
}

func loadSettings(db *sql.DB) (map[string]Setting, error) {
	const cmd = "SELECT Name, Value, DefaultValue from Setting"
	rows, err := db.Query(cmd)

	res := make(map[string]Setting)

	for rows.Next() {
		setting := Setting{}
		if err = rows.Scan(&setting.Name, &setting.Value, &setting.Default); err != nil {
			return nil, err
		}
		res[setting.Name] = setting
	}
	return res, err
}

func deleteSetting(db *sql.DB, key string) error {
	const cmd = "UPDATE Setting set Value = DefaultValue WHERE Name = ?"
	_, err := db.Exec(cmd, key)
	return err
}
