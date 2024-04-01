all: install

build:
	@go build -o ts_autosync main.go
	@sudo mv ts_autosync /usr/local/bin/ts_autosync

install:
	@go build -o ts_autosync main.go
	@sudo mv ts_autosync /usr/local/bin/ts_autosync
	@cp _static/com.charczuk.ts_autosync.plist ${HOME}/Library/LaunchAgents/com.charczuk.ts_autosync.plist
	@defaults write ${HOME}/Library/LaunchAgents/com.charczuk.ts_autosync.plist ProgramArguments -array /usr/local/bin/ts_autosync --source ${HOME}/autosync --rm
	@defaults write ${HOME}/Library/LaunchAgents/com.charczuk.ts_autosync.plist StandardErrorPath "${HOME}/Library/Logs/ts_autosync.log"
	@launchctl load -w ${HOME}/Library/LaunchAgents/com.charczuk.ts_autosync.plist

uninstall:
	@launchctl unload -w ${HOME}/Library/LaunchAgents/com.charczuk.ts_autosync.plist
	@sudo rm /usr/local/bin/ts_autosync