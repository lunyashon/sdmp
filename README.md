# SDMP

Платформа приёма, обогащения и маршрутизации лидов (CDP/DMP). Два процесса: **API** принимает трафик и отдаёт UI, **worker** обогащает лиды и отправляет их наружу.

## Что делает

1. Принимает лиды: `crm.lead.add` (как Битрикс), вебхуки Скорозвона, импорт.
2. Кладёт сырьё в S3, строку — в PostgreSQL, задачу — в outbox.
3. Worker нормализует телефон, ищет регион/оператора, проверяет дубли Зари, режет сплит, пишет партнёрам / Google Sheets / Скорозвон / CRMCalls / Битрикс.

## Состав

| Часть | Назначение |
|---|---|
| `cmd/api` | REST, приём лидов, swagger, статика проксируется nginx |
| `cmd/worker` | outbox: обогащение, партнёры, SK, Bitrix, CRMCalls |
| `frontend/list` | список и карточка лида |
| `frontend/settings` | справочники, сплит, маппинг Bitrix, проекты SK |
| `migrate/` | схема и инкрементальные SQL |
| `deploy/nginx` | `cdp-dmp.ru` → API `:8080` + статика |

## Проверки Зари

Две независимые проверки на обогащении:

- **Проверка по Заре** (`zarya`) — `soda.lead.search`. Результат: Чистый / Дубль. По нему идут партнёры и таблицы.
- **Старая проверка на дубль** (`zarya_old`) — как старый робот: `crm.duplicate.findbycomm` → последний LEAD → `crm.lead.get` → `UF_CRM_1647447791` (`880` Север / `881` Юг) и 90 дней. Пишет только Чистый / дубль. Пустой макрорегион — поле не пишем.

Маппинг UF в Битрикс — в настройках, ключи `zarya` и `zarya_old`.

## Запуск

Нужны PostgreSQL, Redis, файл `config/.env`.

```bash
cp config/.env.example config/.env
# заполнить секреты, в git его нет

docker compose up -d --build api worker
```

Локально без Docker:

```bash
go run ./cmd/api/main.go
go run ./cmd/worker/main.go
```

Миграции из `migrate/` накатывать на базу по порядку дат. Схема целиком: `migrate/sdmp_schema.sql`.

## Конфиг

Все ключи — в `config/.env.example`. Критичные:

- `DB_*`, `REDIS_*`, `YA_S3_*`
- `PHONE_SEARCH_URL`, `PHONE_SEARCH_SECRET_TOKEN`
- `ZARYA_API_URL`, `ZARYA_BITRIX_URL`
- `SKOROZVON_*`, `SODATECH_BX24_URL`, `CRMCALLS_*`
- `SWAGGER_TOKEN`, `CRM_LEAD_ADD_RPS`

`config/.env` в репозиторий не коммитить.

## Тесты

```bash
go test ./...
```

## Репозиторий

Рабочий remote: [github.com/LabDence/sdmp](https://github.com/LabDence/sdmp).
