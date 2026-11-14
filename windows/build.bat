@echo off
rem Build AnticaterKnob.exe on Windows. Needs Go 1.21+ on PATH; no other dependencies.
cd /d "%~dp0"
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -ldflags "-s -w -H=windowsgui" -o AnticaterKnob.exe .
if errorlevel 1 (
  echo Build failed.
  exit /b 1
)
echo Built %cd%\AnticaterKnob.exe
