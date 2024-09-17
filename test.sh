# Build with the pluginload tag
go build -tags pluginload -o gitspace .

# Run tests for both plugins
./gitspace -test-plugin -plugin=hello-world
./gitspace -test-plugin -plugin=templater
