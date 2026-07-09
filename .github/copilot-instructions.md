# Инструкция для ИИ

`http-tester` — Go-утилита для тестирования DNS записей и веб-серверов. Документация: [README.md](../README.md)

## Структура

- `main.go` — точка входа, CLI, оркестрация
- `config/config.go` — типы и парсинг YAML
- `checker/dns/` — DNS проверки (A, AAAA, HTTPS, согласованность)
- `checker/web/` — Web проверки (порт 80, 443, HTTP/2, HTTP/3)
- `report/report.go` — вывод (текст + JSON)

## Ключевые моменты

- Конфиг: один файл = один домен с алиасами
- DNS: `yes`/`no`/`maybe` для проверки наличия/отсутствия записей
- Web: `http: redirect|direct`, `https: true|false`
- IPv4/IPv6 тестируются автоматически
- HTTP/3 работает только при `Alt-Svc: h3` в DNS или ответе
- `*.yaml` в корне в `.gitignore` — не коммитить
- Exit code: 0 = OK, 1 = FAIL. WARN не влияет на код
