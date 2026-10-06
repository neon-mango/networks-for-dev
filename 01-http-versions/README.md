# Мультипротокольный микросервис

Микросервис заметок: key, title, value, updated_at. Свежие выше в списке.

Протоколы: HTTP/0.9, HTTP/1.1 (JSON/XML), HTTP/2 (h2c), HTTP/3 (QUIC/UDP), gRPC.

    go build -o server ./cmd/server && ./server
    go test ./...

Порты: 8080 http/1.1, 8081 http/2, 8082 http/3, 8090 http/0.9, 50051 grpc.

## Запуск

- [HTTP/0.9](#http09)
- [HTTP/1.1 и XML](#http11-и-xml)
- [HTTP/2](#http2)
- [HTTP/3](#http3)
- [gRPC](#grpc)

## HTTP/0.9

Транспорт: raw TCP, порт 8090. Запрос - одна строка пути, ответ - только
тело: без статусной строки, заголовков и методов (всё это появилось
в HTTP/1.0). Доступно только чтение: список и одна заметка.

Список (свежие выше):

    echo '/notes' | nc localhost 8090

Ответ: строки `key\ttitle\tvalue\tupdated_at`, разделитель - табуляция.

Одна заметка:

    echo '/notes/t1' | nc localhost 8090

Ошибки тоже текстом, без статусов:

    echo '/notes/t9' | nc localhost 8090
    # error: note not found

    echo '/bad' | nc localhost 8090
    # error: unknown path

Примечание: nc -w1 закрывает соединение через секунду; обычно ответ
приходит быстрее и соединение закрывается сервером сразу.

## HTTP/1.1 и XML

Транспорт: TCP, порт 8080, кодировка JSON по умолчанию. Ниже - те же
запросы вручную через netcat, чтобы видеть сырой протокол.

Создание заметки. Content-Length обязателен, считаем его длиной переменной:

    BODY='{"key":"k1","title":"t1","value":"v1"}'
    printf "POST /notes HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: ${#BODY}\r\nConnection: close\r\n\r\n$BODY" | nc localhost 8080

Ожидаем: 201 Created и JSON с updated_at.

Список (свежие выше):

    printf 'GET /notes HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n' | nc localhost 8080

Обновление:

    BODY='{"value":"v1 updated"}'
    printf "PUT /notes/t1 HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: ${#BODY}\r\nConnection: close\r\n\r\n$BODY" | nc localhost 8080

Удаление:

    printf 'DELETE /notes/t1 HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n' | nc localhost 8080

Отсутствующая заметка - 404:

    printf 'GET /notes/t9 HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n' | nc localhost 8080

### XML

Формат выбирается заголовками: Accept - для ответа, Content-Type - для
запроса.

Ответ в XML:

    printf 'GET /notes HTTP/1.1\r\nHost: localhost\r\nAccept: application/xml\r\nConnection: close\r\n\r\n' | nc localhost 8080

Запрос в XML (curl короче):

    curl -X POST localhost:8080/notes \
        -H 'Content-Type: application/xml' -H 'Accept: application/xml' \
        -d '<createRequest><key>k2</key><title>t2</title><value>v2</value></createRequest>'

То же самое curl-ом в одну строку для JSON:

    curl -X POST localhost:8080/notes -d '{"key":"k1","title":"t1","value":"v1"}'

Примечание: ${#BODY} считает символы, а не байты. Для кириллицы в JSON длина
в байтах больше: `printf %s "$BODY" | wc -c`.

## HTTP/2

Транспорт: TCP, порт 8081, режим h2c (cleartext, без TLS). Поэтому curl-у
нужен флаг prior knowledge: согласования через ALPN нет.

Список (свежие выше):

    curl --http2-prior-knowledge localhost:8081/notes

Создание, обновление, удаление:

    curl --http2-prior-knowledge -X POST localhost:8081/notes -d '{"key":"k2","title":"t2","value":"v2"}'
    curl --http2-prior-knowledge -X PUT localhost:8081/notes/t2 -d '{"value":"v2 updated"}'
    curl --http2-prior-knowledge -X DELETE localhost:8081/notes/t2

Версия протокола в ответе:

    curl --http2-prior-knowledge -o /dev/null -w '%{http_version}\n' localhost:8081/notes
    # 2

Preface руками через netcat: каждое h2-соединение начинается с фиксированной
строки. Отправим только её - сервер ответит SETTINGS-кадром:

    printf 'PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n' | nc localhost 8081 | od -A x -t x1z | head -4

Первые байты ответа: `00 00 24 04` - заголовок кадра SETTINGS (тип 0x04).

Дальше вручную продолжать нельзя: кадры бинарные, заголовки сжаты HPACK,
состояние кодирования двустороннее. Для просмотра wire-уровня используйте
Wireshark с диссектором http2 - он собирает HPACK-таблицы.

## HTTP/3

Транспорт: QUIC поверх UDP, порт 8082. QUIC требует TLS 1.3, поэтому схема
обязательно https, а self-signed сертификат сервера пропускаем через -k.

Список (свежие выше):

    curl --http3 -k https://localhost:8082/notes

Создание, обновление, удаление:

    curl --http3 -k -X POST https://localhost:8082/notes -d '{"key":"k3","title":"t3","value":"v3"}'
    curl --http3 -k -X PUT https://localhost:8082/notes/t3 -d '{"value":"v3 updated"}'
    curl --http3 -k -X DELETE https://localhost:8082/notes/t3

Версия протокола в ответе:

    curl --http3 -k -o /dev/null -w '%{http_version}\n' https://localhost:8082/notes
    # 3

Типичные ошибки:

- Без схемы (`curl --http3 localhost:8082/notes`) curl пойдет по TCP
  и получит отказ соединения: QUIC живет в UDP.
- Без -k: ошибка проверки сертификата (сервер генерирует self-signed
  при старте).

Низкоуровневый просмотр пакетов: `tcpdump -i any udp port 8082` или
Wireshark с фильтром quic - там виден crypto handshake TLS 1.3 прямо
в QUIC-пакетах.

## gRPC

Транспорт: HTTP/2, порт 50051, без TLS (plaintext). Reflection включен,
поэтому список методов и схемы grpcurl получает с сервера.

Установка grpcurl: `nix-shell -p grpcurl` (или официальный бинарник).

Список сервисов и методов:

    grpcurl -plaintext localhost:50051 list

Создание:

    grpcurl -plaintext -d '{"key":"k1","title":"t1","value":"v1"}' \
        localhost:50051 notes.NotesService/CreateNote

Чтение:

    grpcurl -plaintext -d '{"title":"t1"}' \
        localhost:50051 notes.NotesService/GetNote

Обновление:

    grpcurl -plaintext -d '{"title":"t1","value":"v1 updated"}' \
        localhost:50051 notes.NotesService/UpdateNote

Удаление:

    grpcurl -plaintext -d '{"title":"t1"}' \
        localhost:50051 notes.NotesService/DeleteNote

Список (свежие выше):

    grpcurl -plaintext localhost:50051 notes.NotesService/ListNotes

Отсутствующая заметка - статус NotFound:

    grpcurl -plaintext -d '{"title":"t9"}' \
        localhost:50051 notes.NotesService/GetNote
    # ERROR: Code: NotFound ... note not found

Пустой title - InvalidArgument:

    grpcurl -plaintext -d '{"key":"k9","title":"   ","value":"v9"}' \
        localhost:50051 notes.NotesService/CreateNote
    # ERROR: Code: InvalidArgument ... title is empty

Примечание: данные передаются флагом -d; позиционный аргумент grpcurl
в этой версии не поддерживает.

Один и тот же gRPC-сервер слушает HTTP/2 - проверить можно любым
h2c-клиентом, gRPC это обычные POST-запросы с content-type
application/grpc.
