run: develop
	bin/develop --secret test --server --port 8181 --database db.sqlite --debug --pretty
develop:
	go build -tags Develop -o bin/develop 
release:
	go build -o bin/MangaGetter_unix 
win-amd64:
	GOOS=windows GOARCH=amd64 go build -o bin/MangaGetter-amd64_windows.exe 
