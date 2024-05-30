run: develop
	bin/develop --server --port 8080 --secret test --database db.sqlite --debug --pretty
develop:
	go build -tags Develop -o bin/develop 
release:
	go build -o bin/MangaGetter_unix 
win-amd64:
	GOOS=windows GOARCH=amd64 go build -o bin/MangaGetter-amd64_windows.exe 
