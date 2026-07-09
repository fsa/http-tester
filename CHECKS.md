# Логика проверок http-tester

Последовательность и условия выполнения всех проверок.

## Общая структура

```
Для каждого домена (основной + алиасы):
  1. DNS проверки
  2. Web проверки (если включены)
```

---

## 1. DNS проверки

Выполняются если в конфиге есть секция `dns`.

### 1.1. A и AAAA записи

**Когда:** `dns.a` или `dns.aaaa` не пустые

**Логика:**
```
Резолвим домен через resolver (или системный)

Для A (dns.a = yes/no/maybe):
  yes:  Если A записей нет → FAIL
        Если есть → добавляем в Records
  no:   Если A записи есть → FAIL
        Если нет → OK (ожидаемо)
  maybe: Если A записей нет → помечаем "not found (optional)"
         Если есть → добавляем в Records

Для AAAA (dns.aaaa = yes/no/maybe):
  Аналогично A

Результат: DNSResult { HasA, HasAAAA }
```

**Используется далее:** фильтрация HTTP проверок (если A нет → пропускаем IPv4, если AAAA нет → пропускаем IPv6)

### 1.2. HTTPS DNS запись

**Когда:** `dns.https` не пустой

**Логика:**
```
Запрашиваем HTTPS запись (тип 65) через miekg/dns

dns.https = yes:
  Если записей нет → FAIL
  Если есть → OK + запускаем проверку согласованности (1.3)

dns.https = no:
  Если записи есть → FAIL (найдены, но не должны быть)
  Если нет → OK (ожидаемо)

dns.https = maybe:
  Если записи есть → OK (optional) + запускаем согласованность
  Если нет → OK (optional)
```

### 1.3. Согласованность HTTPS ↔ A/AAAA

**Когда:** HTTPS запись найдена (yes или maybe с результатом)

**Логика:**
```
1. Резолвим A/AAAA для домена
2. Получаем HTTPS записи
3. Для каждой HTTPS записи:
   - Если target = "." (alias mode) → пропускаем
   - Если target указан → резолвим его
   - Проверяем пересечение IP target с IP домена
   - Если нет пересечения → inconsistency
```

---

## 2. Web проверки

Выполняются если в конфиге есть секция `web` с `http` или `https`.

### 2.1. Порт 80 (HTTP/1.1)

**Когда:** `web.http` задан (redirect/direct)

**Логика:**
```
Определяем IP (IPv4, IPv6) для домена

Для каждого доступного IP:
  Отправляем GET http://domain/ (HTTP/1.1)
  Не следуем редиректам (CheckRedirect = ErrUseLastResponse)

  web.http = redirect:
    Ожидаем: 301 или 302
    Если другое → FAIL

  web.http = direct:
    Ожидаем: 200-399
    Если другое → FAIL

  Записываем: redirect_to (если есть Location header)
```

**Результат:** `http-ipv4`, `http-ipv6`

### 2.2. HTTPS (HTTP/2)

**Когда:** `web.https = true`

**Логика:**
```
Определяем IP (IPv4, IPv6)

Для каждого доступного IP:
  Отправляем GET https://domain/ (HTTP/2)
  Не следуем редиректам

  Ожидаем: 200-399
  Если другое → FAIL

  Записываем:
    - alt_svc (из заголовка Alt-Svc)
    - redirect_to (если есть Location header)
```

**Результат:** `https-http2-ipv4`, `https-http2-ipv6`

### 2.3. HTTP/3

**Когда:** `web.https = true` И в HTTP/2 ответе есть `Alt-Svc: h3`

**Логика:**
```
Определяем IP (IPv4, IPv6)

Для каждого доступного IP:
  Подключаемся по QUIC (порт 443)
  Отправляем GET https://domain/ (HTTP/3)

  Ожидаем: 200-399
  Если другое → FAIL

  Записываем: redirect_to (если есть Location header)
```

**Результат:** `https-http3-ipv4`, `https-http3-ipv6`

### 2.4. Предупреждения и информация

#### INFO: Нет HTTPS DNS записи

**Когда:** `web.https = true` И обнаружен HTTP/3 И HTTPS DNS запись не задана в конфиге

**Действие:** Выводим INFO:
```
HTTP/3 supported but no HTTPS DNS record — consider adding https: yes
```

Не влияет на exit code.

#### WARN: Нет Alt-Svc при наличии HTTPS записи

**Когда:** `web.https = true` И HTTPS DNS запись существует И сервер не отдаёт Alt-Svc

**Действие:** Выводим WARN и считаем это ошибкой:
```
HTTPS DNS record exists but server does not advertise Alt-Svc header
```

**Влияет на exit code** (считается как FAIL).

---

## 3. Фильтрация по DNS

После выполнения HTTP проверок, результаты фильтруются:

```
Если HasA = false → удаляем все проверки с "ipv4" в имени
Если HasAAAA = false → удаляем все проверки с "ipv6" в имени
```

---

## 4. Порядок вывода

Для каждого домена:
1. dns (A/AAAA)
2. dns-https
3. dns-consistency (если применимо)
4. http-ipv4 / http-ipv6
5. https-http2-ipv4 / https-http2-ipv6
6. https-http3-ipv4 / https-http3-ipv6
7. dns-https-info (если применимо)
8. http-alt-svc-warn (если применимо)

---

## 5. Exit code

- `0` — все проверки PASS
- `1` — хотя бы одна проверка FAIL или WARN
- INFO не влияет на exit code

---

## Пример конфига и ожидаемого поведения

```yaml
name: example.com
dns:
  a: yes          # A обязательна → иначе FAIL
  aaaa: maybe     # AAAA опционально → просто пропустим IPv6 проверки если нет
  https: yes      # HTTPS обязательна → иначе FAIL + согласованность
web:
  http: redirect  # порт 80 → ожидаем 301/302
  https: true     # порт 443 → HTTP/2 + HTTP/3 (если Alt-Svc: h3)
```

**Порядок проверок:**
1. dns (A + AAAA)
2. dns-https
3. dns-consistency
4. http-ipv4 (порт 80 → 301/302)
5. http-ipv6 (порт 80 → 301/302) — если AAAA есть
6. https-http2-ipv4 (порт 443 → 200)
7. https-http2-ipv6 (порт 443 → 200) — если AAAA есть
8. https-http3-ipv4 (если Alt-Svc: h3)
9. https-http3-ipv6 (если Alt-Svc: h3) — если AAAA есть
10. dns-https-info (если HTTP/3 есть, но dns.https не задан)
11. http-alt-svc-warn (если HTTPS запись есть, но Alt-Svc нет)
