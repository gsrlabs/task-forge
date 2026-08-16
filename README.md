# Task Forge

REST API сервис для управления задачами в командах.

Task Forge предоставляет командную работу с задачами, ролевой моделью доступа, историей изменений, фильтрацией и пагинацией, Redis-кешированием, rate limiting и интеграцией с email-сервисом для приглашения пользователей в команды.

Проект построен на Go и PostgreSQL и запускается в Docker Compose вместе со всеми необходимыми инфраструктурными сервисами.

---

## ✨ Возможности

### 🔐 Аутентификация

- Регистрация пользователей
- JWT-аутентификация
- JWT хранится в `HttpOnly` cookie
- `Secure` cookie в production
- `SameSite=Strict`
- Защита от user enumeration
- Единая ошибка для неверного email и пароля
- Rate limiting для регистрации и авторизации
- Защита от перебора паролей

### 👥 Команды

- Создание команд
- Автоматическое назначение создателя команды в роль `owner`
- Получение списка команд пользователя
- Ролевая модель:
  - `owner`
  - `admin`
  - `member`
- Приглашение существующих пользователей в команду
- Проверка прав на приглашение
- Защита от self-invite
- Защита от повторного добавления пользователя

### 📋 Управление задачами

- Создание задач
- Обновление задач
- Назначение исполнителя
- Снятие исполнителя
- Статусы:
  - `todo`
  - `in_progress`
  - `review`
  - `done`
- Проверка принадлежности пользователя к команде
- Проверка принадлежности исполнителя к команде
- Фильтрация задач
- Пагинация на уровне БД

### 📝 История изменений

Для каждой задачи сохраняется аудит изменений:

- создание задачи;
- изменение задачи;
- пользователь, выполнивший изменение;
- изменённые поля;
- значения `from` / `to`.

История доступна через отдельный API endpoint.

### 📧 Email-приглашения

При добавлении пользователя в команду сервис автоматически отправляет email-приглашение.

Поддерживаются:

- SMTP;
- Mailtrap;
- Console mode для разработки.

Для локальной разработки используется Mailpit.

Email содержит:

- название команды;
- email/имя приглашающего пользователя;
- назначенную роль;
- информацию о приглашении.

Для защиты от недоступности внешнего email-сервиса используется Circuit Breaker.

### ⚡ Redis

Redis используется для:

- кеширования списка задач команды;
- rate limiting.

Кеш списка задач имеет TTL `5 минут`.

При изменении данных, влияющих на кеш списка задач, кеш инвалидируется.

### 🛡️ Rate Limiting

Для защищённых endpoints реализовано два уровня ограничения:

- IP limit — `100 запросов/минуту`;
- per-user limit — `100 запросов/минуту`.

Для публичных endpoints используются отдельные ограничения.

#### Registration

- максимум `10` запросов с одного IP за `2 минуты`;
- максимум `5` запросов на один email за `5 минут`.

#### Login

- максимум `10` попыток с одного IP за `30 минут`;
- максимум `5` попыток на один email за `15 минут`.

Для хранения email в Redis используется HMAC-SHA256, поэтому исходный email не сохраняется в ключах rate limiter.

При недоступности Redis используется fail-open стратегия: запросы продолжают обрабатываться.

### 🔌 Circuit Breaker

При отправке email используется Circuit Breaker.

Если внешний email-сервис становится недоступен, сервис не продолжает бесконечно выполнять неуспешные запросы.

После достижения заданного количества последовательных ошибок Circuit Breaker переходит в состояние `OPEN`.

В этом состоянии новые вызовы email-сервиса блокируются.

После истечения timeout Circuit Breaker переходит в `HALF-OPEN` и проверяет восстановление внешнего сервиса.

Состояния:

```text
CLOSED
   │
   │ несколько последовательных ошибок
   ▼
 OPEN
   │
   │ timeout
   ▼
HALF-OPEN
   │
   ├── success ──► CLOSED
   │
   └── failure ─► OPEN
````

При этом ошибка отправки email не отменяет уже выполненное добавление пользователя в команду.

### 📊 Prometheus Metrics

В приложении реализован сбор HTTP-метрик для последующего мониторинга через Prometheus.

Собираются следующие показатели:

- общее количество HTTP-запросов;
- количество HTTP-запросов с ошибками (`4xx` и `5xx`);
- время выполнения HTTP-запросов.

Основные метрики:

```text
taskforge_http_requests_total
taskforge_http_errors_total
taskforge_http_request_duration_seconds
```
Метрики разбиваются по HTTP-методу, endpoint и HTTP-статусу, что позволяет анализировать нагрузку, количество ошибок и производительность отдельных API-методов.

---

## 🏗️ Архитектура

Проект построен с разделением ответственности между слоями приложения.

```text
                    ┌─────────────────────┐
                    │       Client        │
                    └──────────┬──────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │      HTTP API       │
                    │      Handlers       │
                    └──────────┬──────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │      Services       │
                    │   Business Logic    │
                    └──────┬───────┬──────┘
                           │       │
                ┌──────────┘       └──────────┐
                ▼                             ▼
       ┌─────────────────┐           ┌─────────────────┐
       │  Repositories   │           │ External Services│
       └────────┬────────┘           └────────┬────────┘
                │                             │
                ▼                             ▼
       ┌─────────────────┐           ┌─────────────────┐
       │   PostgreSQL    │           │      Email      │
       └─────────────────┘           │     Service     │
                                     └─────────────────┘

                ┌─────────────────┐
                │      Redis      │
                │ Cache / Limits  │
                └─────────────────┘
```

---

## 🗄️ Модель данных

Основные сущности:

```text
users
  │
  ├──────────────┐
  │              │
  ▼              ▼
teams        team_members
  │              │
  │              │
  ▼              ▼
tasks        users
  │
  ├──────────────► task_history
  │
  └──────────────► task_comments
```

Основные связи:

* `teams.created_by → users.id`
* `team_members.user_id → users.id`
* `team_members.team_id → teams.id`
* `tasks.assignee_id → users.id`
* `tasks.team_id → teams.id`
* `tasks.created_by → users.id`
* `task_history.task_id → tasks.id`
* `task_history.changed_by → users.id`
* `task_comments.task_id → tasks.id`
* `task_comments.user_id → users.id`

---

## 🚀 API

API реализован в формате REST.

### Authentication

| Method | Endpoint           | Description              |
| ------ | ------------------ | ------------------------ |
| `POST` | `/api/v1/register` | Регистрация пользователя |
| `POST` | `/api/v1/login`    | Аутентификация           |

### Teams

| Method | Endpoint                    | Description                   |
| ------ | --------------------------- | ----------------------------- |
| `POST` | `/api/v1/teams`             | Создание команды              |
| `GET`  | `/api/v1/teams`             | Получение команд пользователя |
| `POST` | `/api/v1/teams/{id}/invite` | Приглашение пользователя      |

### Tasks

| Method | Endpoint                     | Description       |
| ------ | ---------------------------- | ----------------- |
| `POST` | `/api/v1/tasks`              | Создание задачи   |
| `GET`  | `/api/v1/tasks`              | Получение задач   |
| `PUT`  | `/api/v1/tasks/{id}`         | Обновление задачи |
| `GET`  | `/api/v1/tasks/{id}/history` | История изменений |

### Analytics

| Method | Endpoint                               | Description                                      |
| ------ | -------------------------------------- | ------------------------------------------------ |
| `GET`  | `/api/v1/analytics/teams/stats`        | Статистика команд                               |
| `GET`  | `/api/v1/analytics/teams/top-creators` | Топ-3 пользователей по созданным задачам       |
| `GET`  | `/api/v1/analytics/integrity/assignees`| Проверка целостности назначенных исполнителей   |


---

## 👤 Ролевая модель

### Owner

Владелец команды.

Получает максимальный уровень управления командой.

### Admin

Администратор команды.

Может приглашать пользователей в команду.

### Member

Обычный участник команды.

Может работать с задачами команды в рамках предоставленных возможностей API.

---

## 📧 Email flow

При приглашении существующего пользователя:

```text
POST /api/v1/teams/{id}/invite
              │
              ▼
      Проверка команды
              │
              ▼
      Проверка прав owner/admin
              │
              ▼
      Поиск пользователя
              │
              ▼
      Проверка существования
      участника в команде
              │
              ▼
      Добавление участника
              │
              ▼
      EmailSender
              │
              ▼
       Circuit Breaker
              │
       ┌──────┴───────┐
       │              │
     CLOSED          OPEN
       │              │
       ▼              ▼
     SMTP          request
    /Mailtrap      rejected
       │
       ▼
     Email
```

Добавление пользователя в команду является основной бизнес-операцией.

Если email-сервис недоступен, участник всё равно остаётся добавленным в команду, а ошибка отправки письма логируется.

---

## ⚡ Кеширование

Для списка задач команды используется Redis.

```text
GET /api/v1/tasks
        │
        ▼
   Redis Cache
        │
   ┌────┴────┐
   │         │
  HIT       MISS
   │         │
   ▼         ▼
 return   PostgreSQL
            │
            ▼
         Redis
            │
            ▼
          return
```

TTL:

```text
5 minutes
```

Кеширование позволяет уменьшить количество повторных запросов к PostgreSQL при частом чтении одного и того же списка задач.

---

## 🗃️ PostgreSQL

Используется PostgreSQL для основного хранения данных.

Реализованы:

* connection pooling;
* индексы для основных запросов;
* пагинация на уровне БД;
* миграции через `goose`;
* сложные SQL-запросы;
* аудит изменений задач.

### Сложные SQL-запросы

Реализованы запросы:

1. Получение статистики по командам с использованием `JOIN` нескольких таблиц и агрегации.

2. Получение топ-3 пользователей по количеству созданных задач в каждой команде за месяц.

3. Поиск задач, у которых назначенный исполнитель не является членом соответствующей команды.

---

## 🔒 Безопасность

### JWT

JWT передаётся через:

```text
HttpOnly Cookie: jwt
```

В production используется:

```text
Secure=true
SameSite=Strict
```

### Защита от user enumeration

API не раскрывает существование пользователя.

Для неверного email и неверного пароля используется единая ошибка:

```text
invalid email or password
```

### Password policy

Пароль должен:

* содержать от `8` до `72` символов;
* содержать минимум одну латинскую букву;
* соответствовать требованиям валидации API.

### Rate limiting

Rate limiting реализован как дополнительный уровень защиты от:

* brute-force;
* abuse;
* чрезмерного количества запросов;
* базовых DDoS-сценариев на уровне приложения.

---

## 🧰 Технологический стек

### Backend

* Go
* REST API
* JWT
* PostgreSQL
* Redis

### Infrastructure

* Docker
* Docker Compose
* PostgreSQL
* Redis
* Mailpit

### Libraries / Components

* `go-redis`
* `gin` / HTTP routing
* `pgx` / PostgreSQL access
* `goose`
* `gobreaker`
* `zerolog`
* Swagger / OpenAPI

---

## 🐳 Docker

Проект запускается через Docker Compose.

В Compose включены основные инфраструктурные компоненты проекта:

```text
Application
    │
    ├── PostgreSQL
    │
    ├── Redis
    │
    └── Mailpit
```

Mailpit используется для локального тестирования отправки email без необходимости подключать настоящий почтовый сервер.

---

## 📖 Swagger

После запуска приложения Swagger UI доступен по адресу:

**Swagger UI**

[http://localhost:8081/swagger](http://localhost:8081/swagger)

Swagger содержит описание REST API, включая:

* authentication;
* teams;
* invitations;
* tasks;
* task history;
* параметры фильтрации;
* pagination;
* схемы запросов и ответов;
* HTTP status codes.

---

## 📬 Mailpit

Для локальной разработки используется Mailpit.

Web UI:

[http://localhost:8025](http://localhost:8025)

После приглашения пользователя в команду письмо можно увидеть в интерфейсе Mailpit.

SMTP:

```text
Host: mailpit
Port: 1025
```

---

## ⚙️ Конфигурация

Конфигурация приложения осуществляется через environment variables.

Основные категории:

```text
APP
DATABASE
REDIS
JWT
EMAIL
SMTP
MAILTRAP
```

Полный список параметров и их значения будут дополнены в инструкции по запуску.

---

## 🚦 Запуск

> Раздел находится в процессе подготовки.

Здесь будет описано:

* необходимые зависимости;
* настройка `.env`;
* запуск Docker Compose;
* запуск миграций;
* проверка состояния сервисов;
* доступ к Swagger;
* доступ к Mailpit;
* локальная разработка;
* автоматический deployment;
* конфигурация production окружения.

---

## 🧪 Тестирование

### Планируется

* Unit-тесты бизнес-логики
* Интеграционные тесты PostgreSQL через Testcontainers
* Минимум `85%` покрытия критических методов

Покрытие и автоматический запуск тестов будут добавлены вместе с CI/CD.

---

## 📊 Prometheus

### Планируется

Будет добавлена интеграция с Prometheus для сбора основных метрик приложения:

* количество HTTP-запросов;
* количество ошибок;
* время ответа;
* распределение latency;
* дополнительные технические метрики сервисов.

---

## 🔄 CI/CD

### Планируется

CI/CD pipeline находится в процессе разработки.

Планируется автоматизировать:

```text
Commit
   │
   ▼
Build
   │
   ▼
Tests
   │
   ▼
Coverage
   │
   ▼
Docker Image
   │
   ▼
Deployment
```

Также планируется добавить автоматический deployment script.

---

## 📌 Roadmap

### Backend

* [x] REST API
* [x] Registration
* [x] JWT authentication
* [x] Team management
* [x] Role-based access
* [x] Team invitations
* [x] Email notifications
* [x] SMTP integration
* [x] Mailtrap integration
* [x] Mailpit integration
* [x] Circuit Breaker
* [x] Task management
* [x] Task filtering
* [x] Pagination
* [x] Task history / audit
* [x] PostgreSQL indexes
* [x] Connection pooling
* [x] Redis caching
* [x] Redis rate limiting
* [x] Goose migrations
* [x] Graceful shutdown

### Security

* [x] HttpOnly JWT cookie
* [x] Secure cookie in production
* [x] SameSite protection
* [x] Password validation
* [x] User enumeration protection
* [x] IP rate limiting
* [x] Per-user rate limiting
* [x] Email HMAC hashing for Redis keys
* [x] Fail-open strategy for Redis rate limiting

### Infrastructure

* [x] Docker
* [x] Docker Compose
* [x] PostgreSQL
* [x] Redis
* [x] Mailpit
* [x] Swagger

### Planned

* [ ] Unit tests
* [ ] PostgreSQL integration tests with Testcontainers
* [ ] Minimum 85% coverage for critical methods
* [ ] Prometheus metrics
* [ ] CI/CD
* [ ] Automated deployment script
* [ ] Complete deployment documentation
* [ ] Production configuration guide

---

## 📁 Project Structure

```text
task-forge
├──cmd
│   └──api
│   │   └──main.go
├──infrastructure
│   ├──postgres
│   │   └──init
│   │   │   └──000-create-app-user.sql
│   └──redis
│   │   └──redis.conf
├──internal
│   ├──cache
│   │   ├──cache.go
│   │   ├──ratelimit.go
│   │   └──tasks.go
│   ├──config
│   │   └──config.go
│   ├──database
│   │   ├──db.go
│   │   ├──migrate.go
│   │   └──pgx_logger.go
│   ├──domain
│   │   ├──analytics.go
│   │   ├──task.go
│   │   ├──team.go
│   │   └──user.go
│   ├──dto
│   │   ├──analytics.go
│   │   ├──auth.go
│   │   ├──dto.go
│   │   ├──email.go
│   │   ├──task.go
│   │   └──team.go
│   ├──email
│   │   ├──email.go
│   │   └──templates.go
│   ├──handler
│   │   ├──analytics.go
│   │   ├──auth.go
│   │   ├──handler.go
│   │   ├──helper.go
│   │   ├──swagger.go
│   │   ├──swagger.yaml
│   │   ├──tasks.go
│   │   └──teams.go
│   ├──middleware
│   │   ├──auth.go
│   │   ├──context.go
│   │   └──middleware.go
│   ├──repository
│   │   ├──analytics.go
│   │   ├──errors.go
│   │   ├──repository.go
│   │   ├──tasks.go
│   │   ├──teams.go
│   │   └──user.go
│   ├──service
│   │   ├──analytics.go
│   │   ├──auth.go
│   │   ├──circuit_breaker.go
│   │   ├──errors.go
│   │   ├──jwt.go
│   │   ├──service.go
│   │   ├──tasks.go
│   │   └──teams.go
│   ├──utils
│   │   └──hash.go
│   └──validator
│   │   └──validator.go
├──migrations
│   └──001_init_schema.sql
├──docker-compose.yml
├──Dockerfile
├──go.mod
├──go.sum
├──README.md
├──.env.example
└──.gitignore
```

---

## 🧠 Архитектурные решения

### PostgreSQL как primary storage

PostgreSQL используется как основной источник истины для пользователей, команд, задач и истории изменений.

### Redis как инфраструктурный слой

Redis используется для данных, которые не должны быть единственным источником истины:

* cache;
* rate limiting.

При недоступности Redis rate limiting работает в fail-open режиме.

### Circuit Breaker для внешних сервисов

Email является внешней зависимостью.

Поэтому его отказ не должен приводить к постоянным повторным попыткам и деградации основного API.

Circuit Breaker изолирует основную бизнес-логику от проблем внешнего сервиса.

### Audit trail

Изменения задач не перезаписывают историю безвозвратно.

Для каждой операции сохраняется информация о том:

* кто изменил задачу;
* когда произошло изменение;
* какие значения были до изменения;
* какие значения стали после изменения.

---

## 📈 Производительность

В проекте предусмотрены несколько уровней оптимизации:

```text
HTTP API
   │
   ├── Rate Limiting
   │
   ▼
Service Layer
   │
   ▼
Redis Cache
   │
   │ cache miss
   ▼
PostgreSQL
   │
   ├── Indexes
   ├── Connection Pool
   └── Pagination
```

Основная цель — минимизировать количество тяжёлых запросов к PostgreSQL и ограничить чрезмерную нагрузку на API.

---

## 🛑 Graceful Shutdown

Приложение поддерживает graceful shutdown.

При получении сигнала завершения приложение корректно завершает работу своих компонентов, позволяя текущим операциям завершиться штатно.

---

