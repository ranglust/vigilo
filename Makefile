.PHONY: build clean run install uninstall help

BINARY_NAME=vigilo
INSTALL_PATH=${HOME}/.local/bin

build:
	go build -o $(BINARY_NAME) main.go

run: build
	./$(BINARY_NAME)

clean:
	go clean
	rm -f $(BINARY_NAME)

install: build
	cp $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	chmod +x $(INSTALL_PATH)/$(BINARY_NAME)

uninstall:
	rm -f $(INSTALL_PATH)/$(BINARY_NAME)

help:
	@echo "Available targets:"
	@echo "  build      - Build the application"
	@echo "  run        - Build and run the application"
	@echo "  clean      - Remove build artifacts"
	@echo "  install    - Install to $(INSTALL_PATH)"
	@echo "  uninstall  - Remove from $(INSTALL_PATH)"
	@echo "  help       - Show this help message"

