# Инструкция для ИИ

## О проекте

`http-tester` — Go-утилита для тестирования DNS записей и доступности веб-сайтов.

## Структура проекта

```
http-tester/
├── main.go                    # Точка входа, CLI парсинг, оркестрация проверок
├── config/
│   └── config.go              # Типы и парсинг YAML конфигурации
├── checker/
│   ├── checker.go             # Интерфейс Checker, типы Result/Record
│   ├── dns/
│   │   ├── dns.go             # A/AAAA проверки, DNSResult
│   │   └── https.go           # HTTPS DNS записи (miekg/dns), согласованность
│   └── http/
│       └── http.go            # HTTP проверки (port 80/443, HTTP/2, HTTP/3)
├── report/
│   └── report.go              # Вывод: текстовый (цветной) и JSON
├── testdata/
│   └── example.yaml           # Пример конфигурации
├── .gitignore                 # Игнорирует *.yaml в корне, бинарник, .mimocode
└── README.md                  # Документация
```

## Формат конфигурации

Один файл = один домен. Пример:

```yaml
name: example.com
dns:
  a: yes        # yes/no/maybe/пусто
  aaaa: yes
  https: yes
web:
  http: redirect   # redirect (301/302) или direct (200)
  https: true      # проверять HTTPS (HTTP/2, HTTP/3 автоматически)
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

### DNS проверки

- `yes` — запись обязательна, иначе FAIL
- `no` — записи не должно быть, иначе FAIL
- `maybe` — опционально: есть = проверяем, нет = пропускаем
- пусто/отсутствие — не проверять

### Web проверки

- `http: redirect` — порт 80 ожидает 301/302
- `http: direct` — порт 80 ожидает 200 OK
- `https: true` — автоматически HTTP/2 + HTTP/3 (если Alt-Svc: h3)

## Ключевые решения

- **DNS**: `github.com/miekg/dns` для HTTPS/SVCB записей (стандартный `net` не поддерживает)
- **HTTP/3**: `github.com/quic-go/quic-go` для QUIC
- **IPv4/IPv6**: тестируются автоматически если есть A и AAAA записи
- **HTTP/3**: тестируется только при `Alt-Svc: h3` в HTTP/2 ответе
- **Редирект**: показывается `redirect_to` в логе и JSON
- **Предупреждения**: жёлтый WARN при отсутствии HTTPS DNS записи при поддержке HTTP/3

## CLI

```bash
./http-tester [флаги] <config.yaml>
```

Порядок аргументов произвольный. Флаги:
- `-resolver <адрес>` — DNS резолвер (порт 53 по умолчанию)
- `-format <text|json>` — формат вывода

## Зависимости

- `gopkg.in/yaml.v3` — парсинг YAML
- `github.com/miekg/dns` — HTTPS DNS записи
- `github.com/quic-go/quic-go` — HTTP/3

## При разработке

- Не коммитить `*.yaml` в корне (в `.gitignore`)
- Не коммитить бинарник `http-tester`
- Exit code: 0 = все PASS, 1 = есть FAIL
- Предупреждения (WARN) не влияют на exit code
