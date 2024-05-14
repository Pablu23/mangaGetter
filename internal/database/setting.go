package database

type Setting struct {
	Name    string `gorm:"PRIMARY_KEY"`
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

//func initSettings(settings *DbTable[string, Setting]) {
//	addSettingIfNotExists("theme", "white", settings)
//	addSettingIfNotExists("order", "title", settings)
//}
//
//func addSettingIfNotExists(name string, value string, settings *DbTable[string, Setting]) {
//	_, exists := settings.Get(name)
//	if !exists {
//		settings.Set(name, NewSetting(name, value))
//	}
//}
