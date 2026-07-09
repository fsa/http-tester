# http-tester

> **⚠️ experimental** — Проект создан с помощью ИИ. Проверяйте код перед использованием в продакшене.

Утилита для тестирования DNS записей и доступности веб-сайтов.

## Возможности

### DNS проверки

- **A записи** — проверка наличия/отсутствия IPv4 адресов
- **AAAA записи** — проверка наличия/отсутствия IPv6 адресов
- **HTTPS записи** (SVCB/HTTPS) — проверка DNS-записей типа HTTPS
- **Согласованность** — автоматическая проверка соответствия HTTPS записей с A/AAAA

### HTTP проверки

- **Порт 80** — HTTP/1.1, проверка редиректа на HTTPS (301/302)
- **Порт 443** — HTTP/2, проверка доступности по HTTPS
- **HTTP/3** — автоматическая проверка при поддержке сервером (Alt-Svc: h3)

### Прочее

- Проверка IPv4 и IPv6 автоматически (если есть A и AAAA записи)
- Кастомный DNS резолвер через `--resolver`
- Вывод в JSON формат через `--format json`
- Предупреждения (WARN) при рекомендациях по настройке

## Использование

```bash
./http-tester [опции] <домен>
./http-tester [опции] -c <config.yaml>
```

По умолчанию принимает имя домена для быстрой проверки. Файл конфигурации передаётся через `-c`.

### Опции

| Длинная | Короткая | Описание |
|---------|----------|----------|
| `--config <файл>` | `-c` | Файл конфигурации |
| `--resolver <адрес>` | `-r` | DNS резолвер. Если порт не указан, используется 53 |
| `--format <формат>` | `-f` | Формат вывода: `text`, `json`, `json-pretty`, `yaml` |

### Примеры

```bash
# Быстрый тест домена (по умолчанию)
./http-tester example.com
./http-tester tavda.info -f json

# С конфигом
./http-tester -c config.yaml

# С кастомным резолвером (порт 53 по умолчанию)
./http-tester -r 8.8.8.8 example.com

# С кастомным резолвером и явным портом
./http-tester --resolver 8.8.8.8:5353 -c config.yaml

# IPv6 резолвер (в квадратных скобках)
./http-tester -r [2001:4860:4860::8888] example.com

# Порядок аргументов произвольный
./http-tester example.com -f json
./http-tester -f json -r 1.1.1.1 example.com
```

## Формат конфигурации

Один файл = один домен с алиасами.

```yaml
name: example.com
dns:
  a: yes        # A запись должна быть
  aaaa: yes     # AAAA запись должна быть
  https: yes    # HTTPS запись должна быть
web:
  http: redirect  # порт 80: redirect (301/302) или direct (200)
  https: true     # проверять HTTPS (HTTP/2, HTTP/3 автоматически)

aliases:
  - name: www.example.com
    dns:
      a: yes
      aaaa: yes
      https: no
    web:
      http: redirect
      https: true
```

### DNS записи

| Значение | Описание |
|----------|----------|
| `yes` | Запись должна существовать, иначе FAIL |
| `no` | Записи не должно быть, если есть — FAIL |
| `maybe` | Опционально: если есть — проверять, если нет — пропустить |
| отсутствие | Не проверять |
| пустая секция `dns:` | Все записи опциональные (как `maybe`) |

### Веб-сервер (web)

| Параметр | Значения | Описание |
|----------|----------|----------|
| `http` | `any` | Порт 80: принимаем 200 или 301/302 (по умолчанию) |
| | `redirect` | Порт 80: ожидаем только 301/302 |
| | `direct` | Порт 80: ожидаем только 200 OK |
| | `no` | Порт 80 не проверяется |
| `https` | `any` | Порт 443: принимаем 200 или 301/302 (по умолчанию) |
| | `redirect` | Порт 443: ожидаем только 301/302 |
| | `direct` | Порт 443: ожидаем только 200 OK |
| | `no` | Порт 443 не проверяется |
| пустая секция `web:` | | `http: any`, `https: any` |

При `https: any/redirect/direct` автоматически выполняются:

1. **HTTP/2** — всегда
2. **HTTP/3** — только если DNS содержит HTTPS запись с alpn=h3 или Alt-Svc: h3

## Пример вывода

```
=== example.com ===
  [PASS] dns: example.com resolved: A: [192.0.2.1], AAAA: [2001:db8::1]
  [PASS] dns-https: found 1 HTTPS record(s)
  [PASS] dns-consistency: consistent
  [PASS] http-http1-ipv4: http://example.com/ -> 302 Moved Temporarily
  [PASS] http-http1-ipv6: http://example.com/ -> 302 Moved Temporarily
  [PASS] https-http2-ipv4: https://example.com/ -> 200 OK
  [PASS] https-http2-ipv6: https://example.com/ -> 200 OK
  [PASS] https-http3-ipv4: https://example.com/ -> 200 OK
  [PASS] https-http3-ipv6: https://example.com/ -> 200 OK

--- Summary ---
All 9 check(s) passed
```

## Сборка

```bash
go build -o http-tester .
```

## Зависимости

- `github.com/miekg/dns` — для HTTPS DNS записей
- `github.com/quic-go/quic-go` — для HTTP/3
