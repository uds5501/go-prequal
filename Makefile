# Define the ports for the servers
SERVER_PORTS := 8081 8082 8083 8084 8085

# Define the config file path for the client
CONFIG_PATH := config.json

# Target to start the servers
start-servers:
	@for port in $(SERVER_PORTS); do \
		echo "Starting server on port $$port"; \
		go run main.go -mode=server -port=$$port & \
	done

# Target to start the client
start-client:
	@echo "Starting client"
	go run main.go -mode=client -config=$(CONFIG_PATH)

# Target to stop all running servers
stop-servers:
	@echo "Stopping all servers"
	@-pkill -f "main.go -mode=server"
	@-pkill -f "/exe/main -mode=server"

# Target to start both servers and client
start-all: start-servers start-client

.PHONY: start-servers start-client start-all