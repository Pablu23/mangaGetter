run: develop
	bin/develop

develop:
	go build -tags Develop -o bin/develop cmd/mangaGetter/main.go

release:
	go build -o bin/MangaGetter_unix cmd/mangaGetter/main.go

win-amd64:
	GOOS=windows GOARCH=amd64 go build -o bin/MangaGetter-amd64_windows.exe cmd/mangaGetter/main.go