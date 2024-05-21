package database

type User struct {
	Id          int `gorm:"primary_key;AUTO_INCREMENT"`
	DisplayName string
	LoginName   string
	PwdHash     []byte
	Salt        []byte
}

// type UserManga struct {
// 	Id          int `gorm:"primary_key;AUTO_INCREMENT"`
// 	DisplayName string
// 	Manga       Manga
// 	User        User
// 	// Chapters    []Chapter `gorm:"ForeignKey:ChapterId,UserId;References:Id,UserId"`
// }
