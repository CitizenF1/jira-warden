# jirawarden

`jirawarden` - это небольшое Go CLI-приложение для создания Jira worklog по merge request из GitLab.
Jira issue key берется из названия MR или ветки по шаблонам вроде `[PCS-123]`.

По умолчанию включен `dry-run`: CLI только показывает, какие списания будут созданы, и не меняет Jira.

## Быстрый Старт

```powershell
$env:GITLAB_URL = "https://gitlab.example.com"
$env:GITLAB_TOKEN = "glpat-..."
$env:GITLAB_USER = "username"
$env:GITLAB_MR_STATE = "all"

$env:JIRA_URL = "https://jira.example.com"
$env:JIRA_EMAIL = "user@example.com"
$env:JIRA_TOKEN = "jira-api-token"
$env:JIRA_AUTH = "auto"
$env:JIRA_ASSIGNEE = "user@example.com"
$env:JIRA_SPRINT_FIELD = "auto"

go run ./cmd/jirawarden -from 2026-06-01 -to 2026-06-18
```

После проверки вывода `dry-run` можно выполнить реальную запись:

```powershell
./jirawarden.exe -from 2026-06-01 -to 2026-06-18 -dry-run=false
```

Перед записью в Jira CLI покажет итоговый список worklog и попросит подтверждение.
Для автоматизации можно использовать `-yes`, чтобы пропустить вопрос.


## Конфигурация

CLI читает настройки из флагов или переменных окружения.

Короткая инструкция по получению токенов: [TOKENS.md](TOKENS.md).

Обязательно для GitLab:

- `GITLAB_URL`
- `GITLAB_TOKEN`
- `GITLAB_USER`
- `GITLAB_MR_STATE`, по умолчанию `all`; значения: `all`, `opened`, `closed`, `locked`, `merged`

Обязательно для реальной записи в Jira:

- `JIRA_URL`
- `JIRA_EMAIL`
- `JIRA_TOKEN`
- `JIRA_AUTH`, по умолчанию `auto`; `basic` для Jira Cloud email/API token, `bearer` для Jira Server/Data Center PAT
- `JIRA_ASSIGNEE`, ожидаемый исполнитель Jira: `accountId`, `name`, email или display name
- `JIRA_SPRINT_FIELD`, custom field спринта в Jira, по умолчанию `auto`

Пример env-файла:

```text
GITLAB_URL=https://gitlab.example.com
GITLAB_TOKEN=glpat-...
GITLAB_USER=username
GITLAB_MR_STATE=all
JIRA_URL=https://jira.example.com
JIRA_EMAIL=user@example.com
JIRA_TOKEN=jira-api-token
JIRA_AUTH=auto
JIRA_ASSIGNEE=user@example.com
JIRA_SPRINT_FIELD=auto
```

В PowerShell можно задать переменные вручную:

```powershell
$env:GITLAB_URL = "https://gitlab.example.com"
$env:GITLAB_TOKEN = "glpat-..."
$env:GITLAB_USER = "username"
$env:GITLAB_MR_STATE = "all"
$env:JIRA_URL = "https://jira.example.com"
$env:JIRA_EMAIL = "user@example.com"
$env:JIRA_TOKEN = "jira-api-token"
$env:JIRA_AUTH = "auto"
$env:JIRA_ASSIGNEE = "user@example.com"
$env:JIRA_SPRINT_FIELD = "auto"
```

## Флаги CLI

```text
-from               дата начала, YYYY-MM-DD
-to                 дата окончания, YYYY-MM-DD
-gitlab-mr-state    состояние GitLab MR: all, opened, closed, locked или merged; по умолчанию all
-dry-run            показать worklog без записи в Jira; по умолчанию true
-yes                записать worklog в Jira без интерактивного подтверждения
-hours-per-day      сколько часов распределять на рабочий день; по умолчанию 8
-max-hours-per-day  максимальный суммарный worklog в Jira за день; по умолчанию 8
-jira-auth          режим авторизации Jira: auto, basic или bearer; по умолчанию auto
-jira-assignee      ожидаемый исполнитель Jira: accountId, name, email или display name
-jira-sprint-field  custom field спринта в Jira или auto; по умолчанию auto
-require-sprint     пропускать задачи вне активного спринта; по умолчанию true
-issue-pattern      дополнительная regexp с одной группой Jira key; можно повторять
-comment-prefix     префикс комментария Jira worklog
-version            вывести версию и выйти
```

## Логика Worklog

- GitLab: берет MR автора `GITLAB_USER` за выбранный период. По умолчанию учитываются все состояния MR.
- Jira issue: извлекает ключ из названия MR по нескольким дефолтным паттернам.
- Несколько разных задач в одном MR поддерживаются: `[PCS-123] [PCS-124] title` создаст contribution для `PCS-123` и для `PCS-124`.
- Если один и тот же ключ случайно указан несколько раз в одном MR, он не задвоится.
- Проверка спринта: по умолчанию время списывается только если задача была в спринте, активном на дату worklog.
- Проверка исполнителя: по умолчанию время списывается только если задача назначена на `JIRA_ASSIGNEE`.
- Дневной лимит: перед записью уже существующие Jira worklog за дату складываются с планируемыми; CLI остановится, если сумма больше `-max-hours-per-day`.
- Распределение: `-hours-per-day` делится поровну между уникальными Jira issue за день.
- Выходные: MR, попавшие на субботу/воскресенье, переносятся на следующий рабочий день, если он попадает в период.

Дефолтные паттерны названий MR:

- `[PCS-123] title`
- `(PCS-123) title`
- `PCS-123 title`
- `[PCS_123] title`
- `[PCS 123] title`
- `#PCS-123 title`
- `JIRA: PCS-123 title`
- `issue PCS-123 title`
- `ticket=PCS-123 title`
- `refs PCS-123 title`
- `feature/PCS-123-title`
- `feat/PCS-123-title`
- `bugfix/PCS-123-title`
- `fix/PCS-123-title`
- `hotfix/PCS-123-title`
- `release/PCS-123-title`
- `task/PCS-123-title`
- `chore/PCS-123-title`
- `refactor/PCS-123-title`
- `docs/PCS-123-title`
- `ci/PCS-123-title`
- `story/PCS-123-title`
- `epic/PCS-123-title`

Дополнительные паттерны можно добавить без удаления дефолтных:

```sh
jirawarden -from 2026-06-01 -to 2026-06-18 -issue-pattern 'JIRA:([A-Z]+-[0-9]+)'
```
