# Сборка И Релиз
 
Этот документ описывает локальную установку, сборку и подготовку релизных архивов `jirawarden`.
 
Важно:
 
- В Windows PowerShell используй `.ps1` команды.
- Команды `sh ./scripts/*.sh` предназначены для macOS/Linux или Git Bash, а не для обычного PowerShell.
 
## Установка На Этот Компьютер
 
Windows PowerShell:
 
```powershell
.\scripts\install.ps1
jirawarden -version
```
 
macOS/Linux:
 
```sh
sh ./scripts/install.sh
jirawarden -version
```
 
Windows-установщик копирует `bin/jirawarden.exe` в `%USERPROFILE%\bin` и добавляет эту папку в пользовательский `PATH`, если нужно. После первой установки перезапусти терминал.
 
macOS/Linux-установщик копирует бинарник в `$HOME/.local/bin`. Убедись, что эта папка есть в `PATH`.
 
## Сборка
 
Windows PowerShell:
 
```powershell
.\scripts\build.ps1 -Version 1.0.1
.\bin\jirawarden.exe -version
```
 
macOS/Linux:
 
```sh
sh ./scripts/build.sh 1.0.1
./bin/jirawarden -version
```
 
## Релиз Для Других Компьютеров
 
Windows PowerShell:
 
```powershell
.\scripts\release.ps1 -Version 1.0.1
```
 
macOS/Linux:
 
```sh
sh ./scripts/release.sh 1.0.1
```
 
Артефакты появятся в `dist/`:
 
- `jirawarden-1.0.1-windows-amd64.zip`
- `jirawarden-1.0.1-linux-amd64.tar.gz`
- `jirawarden-1.0.1-darwin-amd64.tar.gz`
- `jirawarden-1.0.1-darwin-arm64.tar.gz`
 
## Что Отправлять Коллегам
 
Windows:
 
```text
jirawarden-1.0.1-windows-amd64.zip
```
 
macOS Intel:
 
```text
jirawarden-1.0.1-darwin-amd64.tar.gz
```
 
macOS Apple Silicon:
 
```text
jirawarden-1.0.1-darwin-arm64.tar.gz
```
 
## Проверка После Распаковки
 
Windows PowerShell:
 
```powershell
.\jirawarden.exe -version
.\jirawarden.exe -h
```
 
macOS/Linux:
 
```sh
chmod +x ./jirawarden
./jirawarden -version
./jirawarden -h
```
 
Для реального запуска нужны переменные окружения GitLab/Jira, описанные в [../README.md](../README.md).