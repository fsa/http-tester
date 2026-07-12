# http-tester

> **⚠️ experimental** — Проект создан с помощью ИИ. Проверяйте код перед использованием в продакшене.

Утилита для проверки доступности веб-сайта по всем современным сценариям подключения. Она тестирует DNS, HTTP, HTTPS и HTTP/3 через IPv4 и IPv6, помогая обнаружить ошибки конфигурации, которые проявляются только при определённом способе подключения.

В отличие от большинства онлайн-сервисов и простых HTTP-чекеров, `http-tester` проверяет не отдельные технологии, а реальные пути, по которым пользователь может попасть на сайт.

## Почему появилась эта утилита

Проект появился после вполне реальной проблемы. Сайт корректно работал по HTTP, HTTPS и HTTP/2, однако HTTP/3 обслуживался другим виртуальным хостом. В результате часть пользователей попадала совсем не на тот сайт. Большинство существующих инструментов проблему не обнаружили, поскольку проверяли только HTTP и HTTPS.

`http-tester` создавался именно для поиска подобных ошибок конфигурации до того, как их обнаружат пользователи.

## Лицензия

Этот проект распространяется под лицензией [GNU General Public License v3.0](LICENSE).

## Что проверяет

### Сценарии подключения

Утилита моделирует все возможные способы подключения пользователя к сайту:

- IPv4 → HTTP (TCP/80)
- IPv6 → HTTP (TCP/80)
- IPv4 → HTTPS (TCP/443, HTTP/2)
- IPv6 → HTTPS (TCP/443, HTTP/2)
- IPv4 → HTTPS (UDP/443, HTTP/3)
- IPv6 → HTTPS (UDP/443, HTTP/3)

Если сервер или сеть поддерживают соответствующий протокол, выполняются все доступные проверки.

### Тесты DNS

- Проверка записей A, AAAA и HTTPS (RFC 9460)
- Поддержка HTTPS ServiceMode и AliasMode
- Проверка параметров `alpn`
- Сравнение `ipv4hint` и `ipv6hint` с фактическими A/AAAA-записями

### Тесты HTTP

- Проверка HTTPS по HTTP/2 (TCP)
- Проверка HTTPS по HTTP/3 (QUIC)
- Проверка доступности сайта по IPv4 и IPv6
- Возможность проверить все найденные IP-адреса

### Согласованность

- Проверка соответствия HTTPS DNS RR и заголовка Alt-Svc
- Сравнение HTTP-статусов
- Сравнение содержимого страниц, полученных разными способами подключения (грубое сравнение)
- Выявление различий между IPv4, IPv6, HTTP/2 и HTTP/3

### Какие ошибки помогает обнаружить

- Один из вариантов подключения обслуживается другим хостом
- Различные ответы по IPv4 и IPv6
- Некорректные HTTPS RR
- Несогласованность Alt-Svc и HTTPS RR
- Отсутствующие или неверные DNS-записи
- Различия между HTTP/2 и HTTP/3

## Использование

```bash
./http-tester [опции] <домен>
./http-tester [опции] -c <config.yaml>
```

### Опции

| Длинная | Короткая | Описание |
|---------|----------|----------|
| `--config <файл>` | `-c` | Файл конфигурации |
| `--resolver <адрес>` | `-r` | DNS-резолвер |
| `--port <порт>` | `-p` | Порт DNS-резолвера (по умолчанию 53) |
| `--format <формат>` | `-f` | Формат вывода: `text`, `json`, `yaml` |
| `--version` | `-V` | Версия и выход |

### Переопределение конфигурации

Приоритет параметров: CLI → конфигурационный файл → значения по умолчанию

```bash
# DNS
--dns.a <value>          yes/no/optional
--dns.aaaa <value>       yes/no/optional
--dns.https <value>      yes/no/optional

# Веб-сервер
--web.http <value>       any/redirect/direct/no
--web.https <value>      any/redirect/direct/no
--web.test-all-ips       проверять все найденные IP
```

### Примеры

```bash
# Быстрая проверка домена
./http-tester example.com

# Использование конфигурации
./http-tester -c config.yaml

# Проверка через конкретный DNS-резолвер
./http-tester -r 8.8.8.8 example.com

# Переопределение параметров конфигурации
./http-tester -c config.yaml --dns.https yes --web.http redirect

# Проверка всех IP-адресов
./http-tester --web.test-all-ips example.com

# Машиночитаемый вывод
./http-tester -f json example.com
./http-tester -f yaml example.com
```

## Формат конфигурации

Один файл описывает один домен.

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

### DNS

| Значение | Описание |
|----------|----------|
| `yes` | Запись должна существовать |
| `no` | Записи быть не должно |
| `optional` | Проверить только если запись существует |
| пустая секция `dns:` | Все записи считаются `optional` |

### Web

| Параметр | Значения | Описание |
|----------|----------|----------|
| `http` | `any` | Допускается 200 или редирект (по умолчанию) |
| | `redirect` | Ожидается только 301/302 |
| | `direct` | Ожидается только 200 OK |
| | `no` | Проверка отключена |
| `https` | `any` | Допускается 200 или редирект (по умолчанию) |
| | `redirect` | Ожидается только 301/302 |
| | `direct` | Ожидается только 200 OK |
| | `no` | Проверка отключена |
| `test_all_ips` | `true/false` | Проверять каждый найденный IP-адрес |

При проверке HTTPS автоматически выполняются:

1. HTTP/2 (TCP)
2. HTTP/3 (если сервер объявляет поддержку через Alt-Svc)

## Пример вывода

```text
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
    [PASS] no HTTPS records in response

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
go build -ldflags "-X main.version=v1.0.0" -o http-tester .
```

## Зависимости

- `github.com/miekg/dns` — работа с DNS и HTTPS RR
- `github.com/quic-go/quic-go` — HTTP/3 (QUIC)
- `github.com/spf13/pflag` — разбор аргументов командной строки
