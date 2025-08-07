# Evocoders Telegram Bot

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)

Telegram бот для управления клубом Evocoders, реализованный на Go. Помогает модерировать дискуссии, обеспечивает поиск на основе ИИ и генерирует ежедневные сводки чата.

## 🚀 Функции

### Модерация
- ✅ **Управление треками**: Удаляет сообщения не-админов в закрытых для чтения треках
- ✅ **Пересылка сообщений**: Пересылает ответы из закрытых треков в основную тему
- ✅ **Очистка входов/выходов**: Удаляет сообщения о присоединении/покидании для более чистых разговоров

### Функции на основе ИИ
- 🔍 **Поиск инструментов** (`/tools`): Находит релевантные ИИ инструменты по запросам пользователей
- 📚 **Поиск контента** (`/content`): Ищет информацию в специальных темах
- 👋 **Поиск знакомств с участниками клуба** (`/intro`): Предоставляет информацию о участниках клуба
- 📋 **Суммаризация чата**: Создает ежедневные сводки разговоров
  - Автоматически публикуется в настроенное время
  - Ручной запуск с помощью `/trySummarize` (только для админов)

### 🎲 Еженедельные Random Coffee встречи
- **Автоматический опрос участия**: Каждую неделю (настраиваемый день и время в UTC, по умолчанию пятница в 14:00 UTC) бот публикует опрос, спрашивая участников, хотят ли они участвовать в random coffee встречах на следующую неделю.
- **Участие/отказ**: Участники могут легко указать свою доступность, отвечая на опрос. Голоса можно изменить или отозвать до формирования пар.
- **Автоматическое формирование пар**: Бот автоматически генерирует и объявляет пары по расписанию (настраиваемый день и время в UTC, по умолчанию понедельник в 12:00 UTC).
- **Ручное формирование пар**: Администратор также может вручную запустить процесс формирования пар с помощью команды `/tryGenerateCoffeePairs`.
- **Объявление случайных пар**: Бот случайным образом объединяет участвующих участников и объявляет пары в основном чате.
- **Самостоятельная организация встреч**: Объединенные в пары участники сами договариваются о дне, времени и формате встречи.

### Управление профилями пользователей
- 👤 **Команда профиля** (`/profile`): Управление личным профилем
  - Создание и редактирование персональной информации (имя, биография)
  - Публикация профиля в специальной теме "Интро"
  - Поиск профилей других участников клуба

### Управление мероприятиями
- 📅 **Управление мероприятиями**: Отслеживание и организация клубных мероприятий
  - Просмотр предстоящих мероприятий с помощью команды `/events`
  - Просмотр тем и вопросов мероприятий с помощью команды `/topics`
  - Предложение тем или вопросов с помощью команды `/topicAdd`
  - Команды администратора для полного управления жизненным циклом мероприятий
  - Поддержка различных типов и статусов мероприятий
  - Публикация мероприятий с временем начала
  - Организация тем внутри мероприятий

### Административные команды
- 🔐 **Управление мероприятиями** (только для админов):
  - `/eventSetup` - Создание новых мероприятий
  - `/eventStart` - Начало мероприятия
  - `/eventEdit` - Редактирование существующих мероприятий
  - `/eventDelete` - Удаление мероприятий
  - `/showTopics` - Просмотр и управление темами мероприятий
- 👥 **Управление пользователями** (только для админов):
  - `/profilesManager` - Управление профилями пользователей
  - `/code` - Ввод кода верификации для Telegram User Client (задом наперед)
- ⚙️ **Команды тестирования** (только для админов):
  - `/trySummarize` - Ручная генерация сводки чата
  - `/tryCreateCoffeePool` - Ручное создание опроса Random Coffee
  - `/tryGenerateCoffeePairs` - Ручная генерация пар Random Coffee

### Утилиты
- ℹ️ **Помощь** (`/help`): Предоставляет информацию об использовании
- 🚀 **Старт** (`/start`): Показывает приветственное сообщение
- ❌ **Отмена** (`/cancel`): Отменяет любой активный диалог
- 🧩 **Динамические шаблоны**: Настраиваемые ИИ промпты, хранящиеся в базе данных

Для получения более подробной информации об использовании бота используйте команду `/help` в чате с ботом.

## 🔑 Необходимые разрешения бота

Для правильной работы бот должен иметь следующие права администратора в Telegram супергруппе:

- 📌 **Закреплять сообщения**: Необходимо для закрепления объявлений о мероприятиях и важной информации
- 🗑️ **Удалять сообщения**: Необходимо для очистки служебных сообщений и модерации треков

Чтобы назначить эти разрешения, добавьте бота как администратора в вашу группу и включите эти специальные права.

## 💾 База данных

Бот использует PostgreSQL с автоматически инициализируемыми таблицами:

| Таблица | Назначение | Ключевые поля |
|---------|------------|---------------|
| **tg_sessions** | Управляет сессиями Telegram User Client | `id`, `data`, `updated_at` |
| **prompting_templates** | Хранит шаблоны ИИ промптов | `template_key`, `template_text` |
| **users** | Хранит информацию о пользователях | `id`, `tg_id`, `firstname`, `lastname`, `tg_username`, `score`, `has_coffee_ban`, `is_club_member` |
| **profiles** | Хранит данные профилей пользователей | `id`, `user_id`, `bio`, `published_message_id`, `created_at`, `updated_at` |
| **events** | Хранит информацию о мероприятиях | `id`, `name`, `type`, `status`, `started_at`, `created_at`, `updated_at` |
| **topics** | Хранит темы, связанные с мероприятиями | `id`, `topic`, `user_nickname`, `event_id`, `created_at` |
| **random_coffee_polls** | Хранит информацию об опросах Random Coffee | `id`, `message_id`, `telegram_poll_id`, `week_start_date`, `created_at` |
| **random_coffee_participants** | Хранит данные участников опросов | `id`, `poll_id`, `user_id`, `is_participating`, `updated_at` |
| **random_coffee_pairs** | Хранит историю сгенерированных пар Random Coffee | `id`, `poll_id`, `user1_id`, `user2_id`, `created_at` |
| **migrations** | Отслеживает миграции базы данных | `id`, `name`, `timestamp`, `created_at` |

## Сборка исполняемого файла

Для сборки исполняемого файла для Windows используйте следующую команду:

```shell
GOOS=windows GOARCH=amd64 go build -o bot.exe
```

Эта команда создаст исполняемый файл Windows с именем `bot.exe`, который может работать на 64-битных системах Windows.

## Разработка и запуск бота

Запуск бота:

```shell
go run main.go  
```

Сборка проекта:

```shell
go build main.go
```

Команда для обновления зависимостей:

```shell
go mod tidy
```

## ⚙️ Конфигурация

Бот использует переменные окружения для конфигурации. Убедитесь, что все они установлены:

### Базовая конфигурация бота
- `TG_EVO_BOT_TOKEN`: Токен вашего Telegram бота
- `TG_EVO_BOT_SUPERGROUP_CHAT_ID`: ID чата вашей супергруппы
- `TG_EVO_BOT_ADMIN_USER_ID`: ID пользователя для администраторского аккаунта (будет получать уведомления о новых темах)
- `TG_EVO_BOT_OPENAI_API_KEY`: Ключ API OpenAI

### Управление темами
- `TG_EVO_BOT_CLOSED_TOPICS_IDS`: Список ID тем, закрытых для чата (через запятую)
- `TG_EVO_BOT_FORWARDING_TOPIC_ID`: ID темы, куда будут отправляться пересылаемые ответы (0 для основной темы)
- `TG_EVO_BOT_TOOL_TOPIC_ID`: ID темы для базы данных ИИ инструментов
- `TG_EVO_BOT_CONTENT_TOPIC_ID`: ID темы для контента
- `TG_EVO_BOT_INTRO_TOPIC_ID`: ID темы для знакомств и информации об участниках клуба
- `TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID`: ID темы для объявлений

### Telegram User Client
- `TG_EVO_BOT_TGUSERCLIENT_APPID`: App ID Telegram API
- `TG_EVO_BOT_TGUSERCLIENT_APPHASH`: App Hash Telegram API
- `TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER`: Номер телефона для Telegram user client
- `TG_EVO_BOT_TGUSERCLIENT_2FAPASS`: Пароль двухфакторной аутентификации для Telegram user client (если используется 2FA)
- `TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE`: Тип сессии для Telegram User Client. Доступные опции:
  - `file`: Включает файловое хранение (используя `session.json`)
  - `database`: Использует хранение в базе данных (требует действительную `TG_EVO_BOT_DB_CONNECTION`)
  - `memory` или пустая: Использует хранение сессии в памяти (сессия будет потеряна после перезапуска)

### Функция ежедневной суммаризации
- `TG_EVO_BOT_DB_CONNECTION`: Строка подключения PostgreSQL (например, `postgresql://user:password@localhost:5432/dbname`) - база данных будет автоматически инициализирована с необходимыми таблицами
- `TG_EVO_BOT_MONITORED_TOPICS_IDS`: Список ID тем для мониторинга суммаризации (через запятую)
- `TG_EVO_BOT_SUMMARY_TOPIC_ID`: ID темы, где будут публиковаться ежедневные сводки
- `TG_EVO_BOT_SUMMARY_TIME`: Время для запуска ежедневной сводки в 24-часовом формате (например, `03:00` для 3 утра)
- `TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED`: Включить или отключить задачу ежедневной суммаризации (`true` или `false`, по умолчанию `true` если не указано)

### Функция Random Coffee
- `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID`: ID темы, где будут публиковаться опросы и пары Random Coffee
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED`: Включить или отключить задачу еженедельного опроса кофе (`true` или `false`, по умолчанию `true` если не указано)
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME`: Время для отправки еженедельного опроса кофе в 24-часовом формате UTC (например, `14:00` для 14:00 UTC, по умолчанию `14:00` если не указано)
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY`: День недели для отправки опроса (например, `friday`, `monday` и т.д., по умолчанию `friday` если не указано)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED`: Включить или отключить задачу автоматической генерации пар (`true` или `false`, по умолчанию `true` если не указано)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME`: Время для генерации и объявления кофейных пар в 24-часовом формате UTC (например, `12:00` для 12:00 UTC, по умолчанию `12:00` если не указано)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY`: День недели для генерации пар (например, `monday`, `tuesday` и т.д., по умолчанию `monday` если не указано)

В Windows вы можете установить переменные окружения, используя следующие команды в командной строке:

```shell
# Базовая конфигурация бота
set TG_EVO_BOT_TOKEN=your_bot_token_here
set TG_EVO_BOT_OPENAI_API_KEY=your_openai_api_key_here
set TG_EVO_BOT_SUPERGROUP_CHAT_ID=chat_id
set TG_EVO_BOT_ADMIN_USER_ID=admin_user_id

# Управление темами
set TG_EVO_BOT_CLOSED_TOPICS_IDS=topic_id_1,topic_id_2,topic_id_3
set TG_EVO_BOT_FORWARDING_TOPIC_ID=forwarding_topic_id
set TG_EVO_BOT_TOOL_TOPIC_ID=tool_topic_id
set TG_EVO_BOT_CONTENT_TOPIC_ID=content_topic_id
set TG_EVO_BOT_INTRO_TOPIC_ID=intro_topic_id
set TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID=announcement_topic_id

# Telegram User Client
set TG_EVO_BOT_TGUSERCLIENT_APPID=your_app_id
set TG_EVO_BOT_TGUSERCLIENT_APPHASH=your_app_hash
set TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER=your_phone_number
set TG_EVO_BOT_TGUSERCLIENT_2FAPASS=your_2fa_password
set TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE=file

# Функция ежедневной суммаризации
set TG_EVO_BOT_DB_CONNECTION=postgresql://user:password@localhost:5432/dbname
set TG_EVO_BOT_MONITORED_TOPICS_IDS=0,2
set TG_EVO_BOT_SUMMARY_TOPIC_ID=3
set TG_EVO_BOT_SUMMARY_TIME=03:00
set TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED=true

# Функция Random Coffee
set TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID=random_coffee_topic_id
set TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED=true
set TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME=14:00
set TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY=friday
set TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED=true
set TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME=12:00
set TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY=monday
```

Затем запустите исполняемый файл.

## Получение кода верификации

Для получения кода верификации необходимо запустить Telegram User Client.
После первого запуска вы получите этот **код в вашем приложении Telegram**.

Отправьте этот код **ЗАДОМ НАПЕРЕД** с помощью команды /code вашему боту.

После этого ваш бот сможет использовать Telegram User Client и будет автоматически обновлять сессию каждые _30 минут_.

## Запуск тестов

Этот проект включает модульные тесты для обеспечения корректной работы функциональности. Вот различные способы запуска тестов:

### Запуск всех тестов

Для запуска всех тестов в проекте:

```shell
go test ./...
```

Эта команда рекурсивно запустит все тесты во всех пакетах вашего проекта.

### Запуск тестов в конкретном пакете

Для запуска тестов в конкретном пакете:

```shell
go test evo-bot-go/internal/handlers/privatehandlers
```

Или перейдите в каталог пакета и запустите:

```shell
cd internal/handlers/privatehandlers
go test
```

### Запуск конкретного теста

Для запуска конкретной тестовой функции:

```shell
go test -run TestHelpHandler_Name evo-bot-go/internal/handlers/privatehandlers
```

Флаг `-run` принимает регулярное выражение, которое соответствует именам тестовых функций.

### Подробный вывод

Для более подробного вывода тестов добавьте флаг `-v`:

```shell
go test -v ./...
```

### Покрытие кода

Для просмотра покрытия тестами:

```shell
go test -cover ./...
```

Для подробного отчета о покрытии:

```shell
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Это сгенерирует HTML отчет, показывающий, какие строки кода покрыты тестами.

### Тест с обнаружением гонок

Для проверки состояний гонки:

```shell
go test -race ./...
```

### Цветной вывод тестов с gotestsum

Для лучшей видимости с цветным выводом тестов и иконками можно использовать gotestsum:

```shell
# Установка gotestsum
go install gotest.tools/gotestsum@latest

# Запуск тестов с цветным выводом и иконками
gotestsum --format pkgname --format-icons hivis

# Если gotestsum не в вашем PATH, запустите его напрямую
go run gotest.tools/gotestsum@latest --format pkgname --format-icons hivis
```

Для максимальной детализации с цветами и иконками:

```shell
go run gotest.tools/gotestsum@latest --format standard-verbose --format-icons hivis --packages=./... -- -v
```

Это обеспечивает цветной вывод с четкими индикаторами прохождения/неудачи и подробной информацией о тестах.