#!/bin/bash

echo "Building for RISC-V 64-bit architecture..."
GOOS=linux GOARCH=riscv64 CGO_ENABLED=0 go build -o nanokvm-redfish -ldflags="-s -w" main.go

if [ $? -eq 0 ]; then
    echo "Build successful!"
    echo "Binary: nanokvm-redfish"
    file nanokvm-redfish
else
    echo "Build failed!"
    exit 1
fi