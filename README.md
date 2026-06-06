# syncoboard

`syncoboard` is a Terminal User Interface (TUI) application designed to interact with the [Syncoboard API](https://github.com/syncoboard/syncoboard).

## Prerequisites

- Go 1.24 or later

## Install

To install the tool using Go:
```bash
go install github.com/syncoboard/syncoboard-cli@latest
```
After installation, you can run the tool by simply typing:
```bash
syncoboard
```

## Building and Running

To run the application directly:
```bash
go run main.go
```

To build the executable:
```bash
go build -o syncoboard .
./syncoboard
```

## Features

This application provides a command-line interface to interact with your Syncoboard workspace and boards efficiently without leaving your terminal.
