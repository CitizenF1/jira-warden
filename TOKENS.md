# Как Получить Токены

Короткая инструкция для настройки `GITLAB_TOKEN` и `JIRA_TOKEN`.

## GitLab Token

Нужен Personal Access Token пользователя GitLab.

1. Открой GitLab.
2. Нажми на аватар справа сверху.
3. Перейди в `Edit profile`.
4. Открой `Access` -> `Personal access tokens`.
5. Создай token с понятным именем, например `jirawarden`.
6. Укажи срок действия.
7. Выбери scope:
   - минимально: `read_api`;
   - если GitLab не отдает MR с `read_api`, используй `api`.
8. Скопируй token сразу после создания: потом GitLab его больше не покажет.

Для CLI:

```powershell
$env:GITLAB_TOKEN = "glpat-..."
$env:GITLAB_USER = "your.gitlab.username"
```

## Jira Token

Для Jira

1. Открой Персональные токены доступа
2. Нажми `Создать Токен`.
3. Назови token, например `jirawarden`.
4. Скопируй token и сохрани его безопасно.

Для CLI:

```powershell
$env:JIRA_EMAIL = "user@example.com"
$env:JIRA_TOKEN = "jira-api-token"
$env:JIRA_AUTH = "basic"
```

## Минимальные Права

Пользователь, чей token используется, должен иметь права:

- видеть нужные Jira задачи;
- видеть sprint field;
- видеть worklog;
- создавать worklog;
- видеть GitLab merge requests нужных проектов.
