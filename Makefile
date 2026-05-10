.PHONY: tui gui gui-debug

tui:
	go run ./

gui:
	export FIELD_KIT_DEBUG=true
	go run -tags=desktop,dev ./
