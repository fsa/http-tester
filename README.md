# http-tester

> **⚠️ experimental** — Проект создан с помощью ИИ. Проверяйте код перед использованием в продакшене.

Утилита для глубокого анализа DNS-записей и веб-серверов. В отличие от базовых утилит, проверяющих только HTTP-заголовки, проводит кросс-валидацию между настройками веб-сервера и конфигурацией DNS-зоны.

## Лицензия

Этот проект распространяется под лицензией [GNU General Public License v3.0](LICENSE).

## Что проверяет

### DNS HTTPS RR (RFC 9460)

- Валидация HTTPS-записей (ServiceMode / AliasMode)
- Проверка параметров alpn (h2, h3)
- Кросс-проверка ipv4hint/ipv6hint с A/AAAA записями

### HTTP/2 и HTTP/3

- Фактические сетевые запросы по TCP (HTTP/2) и UDP/QUIC (HTTP/3)
- Проверка реальной поддержки протоколов, а не только декларации
- Тестирование доступности по IPv4 и IPv6 при их поддержке сайтом

### Согласованность

- Проверка Alt-Svc заголовков vs HTTPS DNS записи
- Сравнение контента страниц по разным протоколам
- Проверка согласованности HTTP-статусов

### Тестирование всех IP

- Проверка A/AAAA записей с режимами yes/no/optional
- Опция `test_all_ips` для проверки каждого найденного IP-адреса
- Автоматическое определение IPv4/IPv6 возможностей

## Использование

```bash
./http-tester [опции] <домен>
./http-tester [опции] -c <config.yaml>
```

### Опции

| Длинная | Короткая | Описание |
|---------|----------|----------|
| `--config <файл>` | `-c` | Файл конфигурации |
| `--resolver <адрес>` | `-r` | DNS резолвер |
| `--port <порт>` | `-p` | Порт резольвера (по умолчанию 53) |
| `--format <формат>` | `-f` | Формат вывода: `text`, `json`, `yaml` |
| `--version` | `-V` | Версия и выход |

### Переопределение конфигурации

CLI-параметры имеют приоритет над конфиг-файлом и дефолтами: **CLI > конфиг-файл > дефолты**.

```bash
# Параметры DNS
--dns.a <value>          yes/no/optional
--dns.aaaa <value>       yes/no/optional
--dns.https <value>      yes/no/optional

# Параметры веб-сервера
--web.http <value>       any/redirect/direct/no
--web.https <value>      any/redirect/direct/no
--web.test-all-ips       проверять все найденные IP
```

### Примеры

```bash
# Быстрый тест домена
./http-tester example.com

# С конфигом
./http-tester -c config.yaml

# С кастомным резолвером
./http-tester -r 8.8.8.8 example.com

# Переопределение параметров конфига через CLI
./http-tester -c tavda.net.yaml --dns.https yes --web.http redirect

# Проверка всех IP-адресов
./http-tester --web.test-all-ips example.com

# Формат вывода
./http-tester -f json example.com
./http-tester -f yaml example.com
```

## Формат конфигурации

Один файл = один домен.

```yaml
name: example.com
dns:
  a: yes
  aaaa: yes
  https: yes
web:
  http: redirect
  https: any
  test_all_ips: false
```

### DNS записи

| Значение | Описание |
|----------|----------|
| `yes` | Запись должна существовать, иначе FAIL |
| `no` | Записи не должно быть, если есть — FAIL |
| `optional` | Опционально: если есть — проверять, если нет — пропустить |
| пустая секция `dns:` | Все записи опциональные |

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
| `test_all_ips` | `true/false` | Проверять все найденные IP (по умолчанию false) |

При `https: any/redirect/direct` автоматически выполняются:

1. **HTTP/2** — всегда
2. **HTTP/3** — только если Alt-Svc: h3

## Пример вывода

```
Started: 12.07.2026 04:54:04 +05

Testing: tavda.net
  DNS: A(yes), AAAA(yes), HTTPS(yes)
  Web: HTTP(redirect), HTTPS(any)

Test Results
Resolver: 8.8.8.8:53

  DNS:
    [PASS] tavda.net resolved
           A 185.199.108.153
           AAAA 2606:50c0:8000::153
    [PASS] no HTTPS records in response (optional)

  HTTP:
    [PASS] http://tavda.net/ -> 301 Moved Permanently (IPv4)
    [PASS] http://tavda.net/ -> 301 Moved Permanently (IPv6)

  HTTPS:
    [PASS] https://tavda.net/ -> 200 OK (HTTP/2, IPv4)
    [PASS] https://tavda.net/ -> 200 OK (HTTP/2, IPv6)
    [PASS] https://tavda.net/ -> 200 OK (HTTP/3, IPv4)
    [PASS] https://tavda.net/ -> 200 OK (HTTP/3, IPv6)

--- Summary ---
All 6 check(s) passed
```

## Сборка

```bash
# Локальная сборка
go build -o http-tester .

# Сборка релизной версии
go build -ldflags "-X main.version=v1.0-RC3" -o http-tester .
```

## Зависимости

- `github.com/miekg/dns` — DNS резолвер и HTTPS DNS записи
- `github.com/quic-go/quic-go` — HTTP/3
- `github.com/spf13/pflag` — CLI аргументы
