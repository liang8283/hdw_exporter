@echo off
title hdw_exporter
setlocal enabledelayedexpansion
cls

set binFolder=bin\

if not exist %binFolder% (				
	md %binFolder%
)

go clean
go mod download
go build -o %binFolder%\hdw_exporter.exe

echo "build over!"
:exit
pause
