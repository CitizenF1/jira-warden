# Сборка И Релиз

Этот документ описывает локальную установку, сборку и подготовку релизных архивов `jirawarden` для других компьютеров.

## Установка На Этот Компьютер

Windows PowerShell:

```powershell
.\scripts\install.ps1
jirawarden -version
```

Linux/macOS:

```sh
sh ./scripts/install.sh
jirawarden -version
```

Windows-установщик копирует `bin/jirawarden.exe` в `%USERPROFILE%\bin` и добавляет эту папку в пользовательский `PATH`, если нужно. После первой установки перезапусти терминал.

Linux/macOS-установщик копирует бинарник в `$HOME/.local/bin`. Убедись, что эта папка есть в `PATH`.

## Сборка

Windows:

```powershell
.\scripts\build.ps1 -Version 0.1.0
.\bin\jirawarden.exe -version
```

Linux/macOS:

```sh
sh ./scripts/build.sh 0.1.0
./bin/jirawarden -version
```

## Релиз Для Других Компьютеров

Создать архивы для Windows, Linux и macOS:

Windows:

```powershell
.\scripts\release.ps1 -Version 0.1.0
```

Linux/macOS:

```sh
sh ./scripts/release.sh 0.1.0
```

Артефакты появятся в `dist/`:

- `jirawarden-0.1.0-windows-amd64.zip`
- `jirawarden-0.1.0-linux-amd64.tar.gz`
- `jirawarden-0.1.0-darwin-amd64.tar.gz`
- `jirawarden-0.1.0-darwin-arm64.tar.gz`

Передай нужный архив на другой компьютер, распакуй его, добавь папку с бинарником в `PATH` и запусти:

```sh
jirawarden -version
jirawarden -from 2026-06-01 -to 2026-06-18
```

## Быстрая Проверка После Релиза

После распаковки на целевой машине проверь:

```sh
jirawarden -version
jirawarden -h
```

Для реального запуска нужны переменные окружения GitLab/Jira, описанные в [README.md](README.md).
