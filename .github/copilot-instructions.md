# Инструкция для ИИ

`http-tester` — Go-утилита для тестирования DNS записей и веб-серверов. Документация: [README.md](../README.md)

## Структура

- `main.go` — точка входа, CLI, оркестрация
- `config/config.go` — типы и парсинг YAML
- `checker/dns/resolver.go` — DNS-сервис: `net.Resolver` для A/AAAA, `miekg/dns` для HTTPS (тип 65)
- `checker/dns/dns.go` — проверки A/AAAA записей
- `checker/dns/https.go` — проверки HTTPS DNS записей и согласованности RFC 9460
- `checker/web/web.go` — Web проверки (порт 80, 443, HTTP/2, HTTP/3)
- `report/report.go` — вывод (текст + JSON)

## Ключевые моменты

- **Resolver** — единый DNS-сервис. По умолчанию `net.Resolver` (системный). С `-r` — указанный сервер для всех запросов. HTTPS-запросы всегда через `miekg/dns` (тип 65 не поддерживается `net.Resolver`).
- Конфиг: один файл = один домен с алиасами
- DNS: `yes`/`no`/`optional` для проверки наличия/отсутствия записей
- Web: `http: redirect|direct`, `https: true|false`
- IPv4/IPv6 тестируются автоматически
- HTTP/3 работает только при `Alt-Svc: h3` в DNS или ответе
- `*.yaml` в корне в `.gitignore` — не коммитить
- Exit code: 0 = OK, 1 = FAIL. WARN не влияет на код
