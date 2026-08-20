BINARY := e9s
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

PREFIX ?= /usr/local
DESTDIR ?=
BINDIR ?= $(PREFIX)/bin
DATADIR ?= $(PREFIX)/share
INSTALL ?= install

.PHONY: build build-gui build-gui-basic install install-gui install-gui-assets package-deb clean test

build:
	go build $(LDFLAGS) -o $(BINARY) .

build-gui:
	go build -tags "gui vte sourceview" $(LDFLAGS) -o $(BINARY)-gui ./cmd/e9s-gui

build-gui-basic:
	go build -tags gui $(LDFLAGS) -o $(BINARY)-gui ./cmd/e9s-gui

install:
	go install $(LDFLAGS) .

install-gui: build-gui install-gui-assets
	$(INSTALL) -Dm755 $(BINARY)-gui $(DESTDIR)$(BINDIR)/$(BINARY)-gui

install-gui-assets:
	$(INSTALL) -Dm644 packaging/linux/io.github.dostrow.e9s.desktop $(DESTDIR)$(DATADIR)/applications/io.github.dostrow.e9s.desktop
	$(INSTALL) -Dm644 packaging/linux/io.github.dostrow.e9s.metainfo.xml $(DESTDIR)$(DATADIR)/metainfo/io.github.dostrow.e9s.metainfo.xml
	$(INSTALL) -Dm644 assets/icons/io.github.dostrow.e9s.svg $(DESTDIR)$(DATADIR)/icons/hicolor/scalable/apps/io.github.dostrow.e9s.svg
	$(INSTALL) -Dm644 assets/icons/io.github.dostrow.e9s-symbolic.svg $(DESTDIR)$(DATADIR)/icons/hicolor/symbolic/apps/io.github.dostrow.e9s-symbolic.svg
	$(INSTALL) -Dm644 assets/icons/io.github.dostrow.e9s-48.png $(DESTDIR)$(DATADIR)/icons/hicolor/48x48/apps/io.github.dostrow.e9s.png
	$(INSTALL) -Dm644 LICENSE $(DESTDIR)$(DATADIR)/licenses/e9s/LICENSE
	$(INSTALL) -d $(DESTDIR)$(DATADIR)/e9s/fonts $(DESTDIR)$(DATADIR)/licenses/e9s/fonts
	$(INSTALL) -m644 assets/fonts/*.ttf $(DESTDIR)$(DATADIR)/e9s/fonts/
	$(INSTALL) -m644 assets/fonts/licenses/* $(DESTDIR)$(DATADIR)/licenses/e9s/fonts/
	$(INSTALL) -m644 assets/fonts/README.md $(DESTDIR)$(DATADIR)/e9s/fonts/README.md

package-deb:
	packaging/debian/build-debs.sh "$(VERSION)" "$(CURDIR)/dist"

test:
	go test ./...

clean:
	rm -f $(BINARY) $(BINARY)-gui

linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 .

linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-linux-arm64 .

darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 .

windows-amd64:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-windows-amd64.exe .
