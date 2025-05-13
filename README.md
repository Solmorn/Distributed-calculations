# Distriduted-calculations

Считает значения арифметических выражений, поддерживает систему пользователей

## Архитектура


1. Клиент отправляет выражение через REST API
2. Оркестратор разбирает выражение на отдельные операции с учетом приоритетов операций
3. Оркестратор отправляет эти операции как задачи в очередь задач
4. Рабочие агенты берут задачи из очереди через gRPC и обрабатывают их
5. Рабочие агенты отправляют результаты обратно Оркестратору
6. Оркестратор продолжает обработку выражения с полученными результатами
7. Когда все операции завершены, итоговый результат сохраняется в базе данных

## Возможности

- Вычисление арифметических выражений с поддержкой операций: `+`, `-`, `*`, `/`
- Учет приоритетов операций (сначала умножение и деление, затем сложение и вычитание)
- Многопользовательский режим с JWT-аутентификацией
- Масштабируемая архитектура с настраиваемым количеством рабочих агентов
- Хранение истории вычислений для каждого пользователя
- REST API для интеграции с другими системами
- gRPC для внутреннего взаимодействия между сервисами

## Установка

1. Клонируйте репозиторий:
   ```bash
   git clone https://github.com/your-username/distributed_calculator.git
   ```
2. Перейдите в корневую директорию проекта:
   ```bash
   cd .\distributed_calculator_final\
   ```

3. Запустите приложение:
   ```bash
   go run .\cmd\main.go
   ```
   
## Конфигурация

Приложение можно настроить с помощью переменных среды:

| Переменная | Описание | Значение по умолчанию |
|------------|----------|----------------------|
| `COMPUTING_POWER` | Количество рабочих горутин на агента | 3 |
| `TIME_ADDITION_MS` | Время обработки операций сложения (мс) | 100 |
| `TIME_SUBTRACTION_MS` | Время обработки операций вычитания (мс) | 100 |
| `TIME_MULTIPLICATIONS_MS` | Время обработки операций умножения (мс) | 200 |
| `TIME_DIVISIONS_MS` | Время обработки операций деления (мс) | 300 |
| `JWT_SECRET` | Секретный ключ для JWT | "default_jwt_secret_key" |

Пример запуска с настроенными параметрами:
```bash
COMPUTING_POWER=5 TIME_ADDITION_MS=50 JWT_SECRET="my_secure_secret" go run cmd/main.go
```

## API

### Аутентификация

#### Регистрация пользователя

**Запрос:**
```
POST /api/v1/register
```

**Тело:**
```json
{
  "login": "username",
  "password": "secure_password"
}
```

**Ответ (успешный):**
```json
{
  "status": "ok"
}
```

#### Вход в систему

**Запрос:**
```
POST /api/v1/login
```

**Тело:**
```json
{
  "login": "username",
  "password": "secure_password"
}
```

**Ответ (успешный):**
```json
{
  "token": "eyJhbGciOiJIUzI..."
}
```

### Вычисление выражений

Все запросы к этим эндпоинтам требуют JWT-токена в заголовке:
```
Authorization: Bearer <ваш_токен>
```

#### Вычисление выражения

**Запрос:**
```
POST /api/v1/calculate
```

**Тело:**
```json
{
  "expression": "2+3*4"
}
```

**Ответ:**
```json
{
  "id": 1
}
```

#### Получение всех выражений пользователя

**Запрос:**
```
GET /api/v1/expressions
```

**Ответ:**
```json
{
  "expressions": [
    {
      "id": 1,
      "expression": "2+3*4",
      "status": "completed",
      "result": 14
    },
    {
      "id": 2,
      "expression": "10/2+5",
      "status": "processing",
      "result": 0
    }
  ]
}
```

#### Получение выражения по ID

**Запрос:**
```
GET /api/v1/expressions/{id}
```

**Ответ:**
```json
{
  "expression": {
    "id": 1,
    "expression": "2+3*4",
    "status": "completed",
    "result": 14
  }
}
```

## Примеры использования

### Типичный сценарий использования

1. Регистрация пользователя:
```bash
curl --location 'http://localhost:8080/api/v1/register' \
--header 'Content-Type: application/json' \
--data '{
  "login": "user1",
  "password": "password123"
}'
```

2. Вход в систему:
```bash
curl --location 'http://localhost:8080/api/v1/login' \
--header 'Content-Type: application/json' \
--data '{
  "login": "user1",
  "password": "password123"
}'
```

Сохраните полученный токен.

3. Вычисление выражения:
```bash
curl --location 'http://localhost:8080/api/v1/calculate' \
--header 'Authorization: Bearer <ваш_токен>' \
--header 'Content-Type: application/json' \
--data '{
  "expression": "2+3*4"
}'
```

4. Получение результата:
```bash
curl --location 'http://localhost:8080/api/v1/expressions/1' \
--header 'Authorization: Bearer <ваш_токен>'
```

### Обработка ошибок

#### Деление на ноль:
```bash
curl --location 'http://localhost:8080/api/v1/calculate' \
--header 'Authorization: Bearer <ваш_токен>' \
--header 'Content-Type: application/json' \
--data '{
  "expression": "5/0"
}'
```

Проверка результата:
```bash
curl --location 'http://localhost:8080/api/v1/expressions/2' \
--header 'Authorization: Bearer <ваш_токен>'
```

Ответ:
```json
{
  "expression": {
    "id": 2,
    "expression": "5/0",
    "status": "error",
    "result": 0
  }
}
```

#### Неверное выражение:
```bash
curl --location 'http://localhost:8080/api/v1/calculate' \
--header 'Authorization: Bearer <ваш_токен>' \
--header 'Content-Type: application/json' \
--data '{
  "expression": "2++3"
}'
```

## Тестирование

go test ./...


