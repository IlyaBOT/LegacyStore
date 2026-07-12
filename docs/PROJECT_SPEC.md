# LegacyStore — полное техническое задание

## 1. Краткое описание проекта

**LegacyStore** — это каталог, веб-сервис и нативный клиент для загрузки программ под старые версии Intel Mac OS X / macOS.

Проект ориентирован на системы:

- Mac OS X 10.5 Leopard Intel;
- Mac OS X 10.6 Snow Leopard Intel;
- OS X 10.7 Lion;
- OS X 10.8 Mountain Lion;
- OS X 10.9 Mavericks;
- OS X 10.10 Yosemite;
- OS X 10.11 El Capitan;
- macOS 10.12 Sierra;
- macOS 10.13 High Sierra;
- macOS 10.14 Mojave, опционально;
- macOS 10.15 Catalina.

PPC, Mac OS Classic, System 7–Mac OS 9 и Apple Silicon не входят в первую версию проекта.

Главная цель LegacyStore — не просто скачивание файлов, а правильный подбор последней совместимой версии приложения под конкретную версию macOS, архитектуру и битность системы.

Проект состоит из следующих частей:

- нативный клиент для старых macOS;
- backend API;
- база данных приложений, версий и файлов;
- веб-интерфейс для пользователей, загрузки файлов, модерации и администрирования;
- файловое хранилище;
- поддержка внешних зеркал;
- система проверки SHA-256;
- будущая поддержка P2P-загрузки с HTTP fallback;
- инструменты извлечения метаданных из `.app`, `.dmg`, `.pkg`, `.zip`;
- скрипты сборки и тестирования;
- Docker Compose окружение для быстрого локального запуска.

---

## 2. Название проекта

Основное название:

```text
LegacyStore
```

Не использовать в названии проекта:

```text
App Store
AppStore
Apple Store
Mac App Store
```

Допустимо использовать формулировки:

```text
legacy macOS software catalog
software catalog for old Intel Macs
software downloader for legacy Mac OS X
```

---

## 3. Общие принципы проекта

### 3.1. Главный принцип совместимости

Каждое приложение может иметь несколько версий, а каждая версия может иметь несколько файлов-артефактов.

Например:

```text
App: Pixelmator
├── Version 1.6
│   ├── Artifact: Pixelmator_1.6_10.5_i386.dmg
│   └── Artifact: Pixelmator_1.6_10.6_universal.dmg
├── Version 2.2
│   └── Artifact: Pixelmator_2.2_10.7_x86_64.dmg
└── Version 3.6
    └── Artifact: Pixelmator_3.6_10.9_10.13_x86_64.dmg
```

Клиент должен показывать пользователю:

- последнюю совместимую версию;
- более старые совместимые версии;
- новые несовместимые версии с объяснением причины;
- несовместимые приложения с понятным визуальным статусом.

### 3.2. Сервер должен думать за клиента

Старые Mac не должны получать огромный JSON-каталог и фильтровать его локально.

Backend должен выполнять:

- фильтрацию по версии macOS;
- фильтрацию по архитектуре;
- фильтрацию по битности;
- подбор рекомендуемой версии;
- подготовку короткого JSON-ответа;
- пагинацию;
- генерацию человекочитаемых причин несовместимости.

### 3.3. Старый клиент должен быть лёгким

Нативный клиент должен быть максимально простым:

- минимум зависимостей;
- компактные JSON-ответы;
- нативный AppKit UI;
- минимум тяжёлой логики на клиенте;
- локальный cache;
- фоновая загрузка файлов;
- безопасная проверка скачанных файлов.

---

## 4. Целевые платформы

### 4.1. Нативный клиент

| Параметр | Требование |
|---|---|
| Платформа | Intel Mac |
| Минимальная ОС | Mac OS X 10.5 |
| Максимальная целевая ОС | macOS 10.15 |
| Архитектуры | i386, x86_64 |
| PPC | Не входит в первую версию |
| Apple Silicon | Не входит в первую версию |
| Язык | Objective-C |
| UI | AppKit |
| Swift | Запрещён для legacy-клиента |
| SwiftUI | Запрещён |
| Storyboards | Запрещены |
| ARC | Не использовать как обязательную зависимость |
| Auto Layout | Не использовать как обязательную зависимость |

### 4.2. Backend

| Параметр | Требование |
|---|---|
| Язык | Go |
| Node.js | Не использовать |
| База данных | PostgreSQL |
| API | REST JSON |
| Формат ответов | Компактный JSON |
| Запуск локально | Docker Compose |
| Reverse proxy | Nginx или Caddy опционально |

### 4.3. Web UI

| Параметр | Требование |
|---|---|
| Основная задача | Аккаунты, загрузка файлов, модерация, админка |
| Стиль | OS X Mavericks / classic Mac App Store-inspired |
| HTTPS | Обязательно для авторизованных страниц |
| Upload файлов | Только через web UI |
| Legacy client upload | Запрещён |

---

## 5. Нативный клиент

### 5.1. Технологии клиента

Клиент должен быть написан на:

```text
Objective-C + AppKit
```

Предпочтительная структура:

```text
legacy-client/
├── LegacyStore.xcodeproj
├── LegacyStore/
│   ├── AppDelegate.h
│   ├── AppDelegate.m
│   ├── MainWindowController.h
│   ├── MainWindowController.m
│   ├── Views/
│   ├── Controllers/
│   ├── Models/
│   ├── Networking/
│   ├── Downloads/
│   ├── Compatibility/
│   ├── Cache/
│   └── Resources/
└── LegacyStoreDownloader/
```

Разрешённые подходы:

- AppKit;
- XIB/NIB;
- manual retain/release;
- CFNetwork;
- libcurl;
- SQLite;
- vendored lightweight JSON parser;
- helper process для загрузчика.

Запрещённые подходы:

- Swift;
- SwiftUI;
- Storyboards;
- Electron;
- Node.js;
- WebView-only клиент;
- обязательный Auto Layout;
- современные API, недоступные на Mac OS X 10.5, без fallback.

### 5.2. Архитектуры клиента

Клиент должен собираться как universal binary:

```text
i386 + x86_64
```

Ожидаемое поведение:

| Система | Архитектура |
|---|---|
| Mac OS X 10.5 Intel | i386 |
| Mac OS X 10.6 Intel | i386 или x86_64 |
| OS X 10.7–10.14 | i386 или x86_64 |
| macOS 10.15 | только x86_64 |

Сборочные скрипты не должны:

- молча удалять `i386`;
- молча повышать deployment target;
- подменять целевую ОС;
- считать сборку успешной, если нужная архитектура не собрана.

Если текущий Xcode не может собрать i386/10.5, скрипт должен вывести понятную ошибку.

### 5.3. Основные экраны клиента

Клиент должен содержать:

```text
Featured
Top Charts
Categories
Downloads
Updates
Search
App Detail Page
```

### 5.4. Главная страница

Главная страница должна содержать:

- верхний toolbar;
- боковую панель;
- Featured-блок;
- список рекомендованных приложений;
- список новых приложений;
- список популярных приложений;
- карточки приложений.

Карточка приложения должна содержать:

- иконку;
- название;
- категорию;
- рейтинг;
- краткое описание;
- рекомендуемую версию;
- бейджи архитектуры;
- статус совместимости;
- кнопку загрузки или просмотра.

### 5.5. Категории

Минимальный набор категорий:

```text
Audio & Video
Business
Developer Tools
Education
Entertainment
Games
Graphics & Design
Internet & Network
Lifestyle
Music
Photography
Productivity
Reference
Social Networking
Utilities
```

### 5.6. Поиск

Поиск должен работать серверной фильтрацией.

Клиент отправляет:

```http
GET /api/v1/search?q=<query>&os=<version>&arch=<arch>&limit=<n>&page=<n>
```

Поиск должен поддерживать:

- название приложения;
- developer name;
- bundle id;
- категории;
- теги;
- описание.

### 5.7. Страница приложения

Страница приложения должна показывать:

- иконку;
- название;
- developer name;
- категорию;
- рейтинг;
- скриншоты;
- описание;
- выбранную рекомендуемую версию;
- список совместимых версий;
- список всех версий;
- причины несовместимости;
- размер файла;
- SHA-256;
- тип файла;
- источник файла;
- лицензионные заметки;
- отзывы;
- кнопки загрузки.

### 5.8. Ручной выбор версии

Пользователь должен иметь возможность вручную выбрать совместимую версию.

Если версия несовместима, клиент может показать её в списке, но должен:

- пометить её как несовместимую;
- отключить кнопку загрузки или потребовать явного подтверждения, если политика проекта это разрешает;
- показать причину несовместимости.

### 5.9. Отображение несовместимости

В списке приложений несовместимое приложение отображается так:

- иконка в grayscale;
- поверх иконки запрещающий overlay;
- подпись `Не совместимо`;
- кнопка загрузки отключена;
- карточка приложения остаётся открываемой.

На странице приложения показывается сообщение:

```text
Этот продукт не совместим с вашим Macintosh.
```

Далее выводятся причины:

```text
Версия системы 10.9 ниже минимальной требуемой продуктом 10.15.
```

```text
Версия системы 15.0 выше максимальной поддерживаемой продуктом 10.5.2.
```

```text
Архитектура вашего Mac i386 не поддерживается. Требуется: x86_64.
```

```text
Продукт является 32-битным, а macOS 10.15 поддерживает только 64-битные приложения.
```

---

## 6. Загрузчик клиента

### 6.1. Основной принцип

Клиент никогда не должен автоматически открывать скачанный файл.

Правильная схема:

```text
скачать → проверить SHA-256 → поставить статус → показать уведомление → ждать действия пользователя
```

### 6.2. Источники загрузки

Клиент должен поддерживать:

- HTTP/HTTPS загрузку;
- несколько HTTP mirrors;
- external direct URL;
- external page URL;
- future P2P через BitTorrent;
- future HTTP fallback после P2P.

### 6.3. P2P и HTTP fallback

Будущая логика:

```text
1. Если доступен torrent/magnet, попробовать P2P.
2. Если P2P недоступен, нет peers, tracker не отвечает или истёк timeout, перейти на HTTP fallback.
3. Если основной HTTP URL недоступен, попробовать зеркала по priority.
4. После загрузки проверить SHA-256.
```

Для P2P желательно рассмотреть:

```text
libtransmission
```

Допустимо реализовать P2P позднее через отдельный helper process.

### 6.4. Статусы загрузки

Клиент должен хранить статусы:

```text
queued
downloading
paused
verifying
downloaded
hash_mismatch
failed
cancelled
```

Пользовательские русские статусы:

```text
В очереди
Загружается
Пауза
Проверка
Загружено
Ошибка хэш-суммы
Ошибка загрузки
Отменено
```

### 6.5. Проверка SHA-256

После завершения загрузки клиент должен:

1. вычислить SHA-256;
2. сравнить с ожидаемым SHA-256 из API;
3. сохранить ожидаемый и фактический hash в локальную базу;
4. показать статус.

Если hash совпал:

- поставить статус `Загружено`;
- показать уведомление, если окно свёрнуто или не в фокусе;
- разрешить действия пользователя.

Если hash не совпал:

- файл не удалять;
- поставить статус `Ошибка хэш-суммы`;
- сохранить файл на диске;
- сохранить expected hash;
- сохранить actual hash;
- показать предупреждение;
- не открывать файл без дополнительного подтверждения.

### 6.6. Поведение при несовпадении SHA-256

При попытке открыть файл с несовпавшим hash показывать предупреждение:

```text
Хэш-сумма загруженного файла не совпадает с хэш-суммой в базе LegacyStore.

Это может означать, что файл был повреждён при загрузке, зеркало отдаёт другую версию файла, либо содержимое было подменено.

Не рекомендуется открывать этот файл.
```

Кнопки:

```text
Перекачать
Показать в Finder
Открыть всё равно
Отмена
```

Кнопка `Открыть всё равно` не должна быть кнопкой по умолчанию.

Действие `Открыть всё равно` должно логироваться локально.

### 6.7. Открытие скачанного файла

Открытие допускается только по явному действию пользователя:

- двойной клик;
- кнопка `Открыть`;
- контекстное меню `Открыть`;
- контекстное меню `Показать в Finder`.

Поведение:

| Тип файла | Действие |
|---|---|
| `.dmg` | смонтировать через `hdiutil` |
| `.iso` | попробовать смонтировать через `hdiutil` |
| `.pkg` | открыть через Installer |
| `.zip` | открыть системным обработчиком архивов |
| `.app` | предпочтительно показать в Finder |
| неизвестный тип | показать в Finder |

Автоматически после скачивания не открывать ничего.

---

## 7. Локальная база клиента

Клиент должен использовать SQLite для cache и истории загрузок.

Минимальная таблица загрузок:

```sql
CREATE TABLE downloads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    artifact_id TEXT NOT NULL,
    app_slug TEXT NOT NULL,
    app_name TEXT NOT NULL,
    version TEXT NOT NULL,
    file_name TEXT NOT NULL,
    local_path TEXT NOT NULL,
    source_type TEXT NOT NULL,
    expected_sha256 TEXT,
    actual_sha256 TEXT,
    status TEXT NOT NULL,
    size_bytes INTEGER,
    downloaded_bytes INTEGER,
    created_at TEXT,
    completed_at TEXT,
    last_error TEXT
);
```

Желательно добавить таблицы:

```sql
CREATE TABLE cached_apps (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    json_payload TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE cached_categories (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);
```

---

## 8. Backend

### 8.1. Технологии backend

Backend должен быть написан на Go.

Рекомендуемый стек:

```text
Go
net/http
chi or standard router
PostgreSQL
Docker Compose
```

Node.js не использовать.

Backend должен быть:

- простым;
- эффективным;
- многопоточным;
- с компактными JSON-ответами;
- с пагинацией;
- с серверной фильтрацией;
- с понятной структурой API.

### 8.2. Структура backend

```text
backend/
├── cmd/
│   └── legacystore-backend/
│       └── main.go
├── internal/
│   ├── api/
│   ├── auth/
│   ├── catalog/
│   ├── compatibility/
│   ├── config/
│   ├── db/
│   ├── downloads/
│   ├── moderation/
│   ├── reviews/
│   ├── storage/
│   └── users/
├── migrations/
├── fixtures/
├── go.mod
└── go.sum
```

### 8.3. Требования к API

API должен:

- отдавать JSON;
- использовать пагинацию;
- не отдавать огромные ответы;
- использовать server-side compatibility filtering;
- не заставлять старый клиент делать тяжёлый post-processing;
- отдавать человекочитаемые причины несовместимости;
- поддерживать ETag/Last-Modified для cache;
- иметь стабильную версию `/api/v1`.

### 8.4. Общие правила JSON

JSON должен быть компактным и простым.

Не использовать чрезмерно вложенные структуры там, где можно упростить.

Не отдавать:

- огромные описания в списке приложений;
- все версии каждого приложения в общем списке;
- все скриншоты в списке приложений;
- все отзывы в списке приложений.

Список приложений должен содержать только карточки.

Полная информация отдаётся на странице приложения.

---

## 9. Сущности базы данных

### 9.1. users

```text
id
email
password_hash
nickname
avatar_url
created_at
updated_at
email_verified
two_factor_enabled
status
```

### 9.2. roles

```text
id
name
```

Допустимые роли:

```text
guest
user
trusted
moder
admin
```

### 9.3. user_roles

```text
user_id
role_id
```

Ограничение:

```text
Только один пользователь может иметь роль admin.
```

### 9.4. apps

```text
id
slug
name
bundle_id
developer_name
summary
description
website_url
license_type
created_by
moderation_status
created_at
updated_at
```

### 9.5. categories

```text
id
slug
name
sort_order
```

### 9.6. app_categories

```text
app_id
category_id
```

### 9.7. app_versions

```text
id
app_id
version
release_date
changelog
is_recommended
created_at
updated_at
```

### 9.8. artifacts

```text
id
app_version_id
file_name
package_type
source_type
storage_path
primary_download_url
torrent_url
magnet_url
size_bytes
sha256
min_os
max_supported_os
max_tested_os
hard_block_above_max
arch_i386
arch_x86_64
supports_32bit
supports_64bit
requires_rosetta
requires_java
install_notes
moderation_status
created_at
updated_at
```

Допустимые `package_type`:

```text
app
dmg
zip
pkg
iso
other
```

Допустимые `source_type`:

```text
local
external_direct
external_page
```

### 9.9. artifact_mirrors

```text
id
artifact_id
mirror_type
url
priority
is_active
```

Допустимые `mirror_type`:

```text
http
official
external_page
web_seed
```

### 9.10. icons

```text
id
app_id
app_version_id nullable
image_url
min_os
max_os
width
height
```

Иконки могут отличаться для разных версий приложения.

### 9.11. screenshots

```text
id
app_id
app_version_id nullable
image_url
min_os
max_os
caption
sort_order
```

Скриншоты могут быть привязаны:

- ко всему приложению;
- к конкретной версии;
- к диапазону macOS.

По умолчанию для скриншота:

```text
min_os = 10.5
max_os = 15
```

### 9.12. reviews

```text
id
app_id
user_id
rating
title
body
created_at
updated_at
deleted_at
```

### 9.13. review_replies

```text
id
review_id
user_id
body
created_at
updated_at
deleted_at
```

### 9.14. review_likes

```text
review_id
user_id
created_at
```

### 9.15. moderation_queue

```text
id
entity_type
entity_id
submitted_by
status
moderator_id
moderator_comment
created_at
updated_at
```

Допустимые статусы:

```text
pending
approved
rejected
```

### 9.16. audit_log

```text
id
actor_user_id
action
entity_type
entity_id
ip_address
user_agent
created_at
```

---

## 10. Роли и права

### 10.1. guest

Незарегистрированный пользователь.

Может:

- смотреть каталог;
- искать приложения;
- открывать страницы приложений;
- читать отзывы;
- скачивать публичные файлы, если политика проекта это разрешает.

Не может:

- писать отзывы;
- ставить лайки;
- загружать файлы;
- модерировать;
- управлять аккаунтами.

### 10.2. user

Обычный пользователь.

Может:

- всё, что guest;
- писать отзывы;
- отвечать на отзывы;
- ставить лайки;
- использовать legacy client auth через app-specific password;
- иметь профиль.

Не может:

- загружать софт;
- редактировать карточки приложений;
- модерировать.

### 10.3. trusted

Доверенный пользователь.

Может:

- всё, что user;
- загружать файлы через web UI;
- предлагать новые приложения;
- предлагать новые версии;
- редактировать свои pending submissions.

Не может:

- публиковать файлы без модерации;
- управлять пользователями;
- выдавать роли.

### 10.4. moder

Модератор.

Может:

- всё, что trusted;
- одобрять или отклонять загруженные файлы;
- редактировать карточки приложений;
- удалять отзывы;
- управлять пользователями в рамках разрешённых прав;
- выдавать роли `user` и `trusted`;
- просматривать moderation queue;
- просматривать audit log, если это разрешено настройками.

Не может:

- создавать второго admin;
- отключать admin;
- менять критические настройки сервера без admin.

### 10.5. admin

Главный администратор.

Может:

- всё;
- управлять ролями;
- управлять системными настройками;
- управлять storage;
- управлять OAuth;
- управлять moderation;
- управлять пользователями.

Ограничение:

```text
Роль admin должна быть единственной.
```

---

## 11. Compatibility engine

### 11.1. Назначение

Compatibility engine определяет, совместим ли artifact с системой пользователя.

Он должен учитывать:

- версию macOS;
- минимальную поддерживаемую версию macOS;
- максимальную поддерживаемую версию macOS;
- максимальную протестированную версию macOS;
- архитектуру;
- битность;
- macOS 10.15 и отсутствие 32-bit app support;
- дополнительные требования.

### 11.2. Входные данные

```json
{
  "os_version": "10.9.5",
  "arch": "x86_64",
  "supports_32bit": true,
  "supports_64bit": true,
  "machine_model": "Macmini7,1"
}
```

### 11.3. Выходные данные

```json
{
  "status": "compatible",
  "level": "recommended",
  "reasons": []
}
```

### 11.4. Статусы

```text
compatible
recommended
probably_compatible
untested
blocked
```

### 11.5. Коды причин

```text
os_too_old
os_too_new
arch_mismatch
bitness_mismatch
requires_32bit
requires_64bit
requires_rosetta
requires_java
unknown_compatibility
untested_newer_os
```

### 11.6. Сравнение версий macOS

Запрещено сравнивать версии как строки.

Нужно корректно сравнивать:

```text
10.5
10.5.2
10.6.8
10.9.5
10.13.6
10.15
11
12
15
```

Пример неправильного сравнения:

```text
"10.15" < "10.9"
```

Так делать нельзя.

Версии нужно парсить в числовые компоненты:

```text
10.15.0
10.9.5
15.0.0
```

### 11.7. Логика подбора рекомендуемой версии

Для конкретного приложения backend должен выбрать лучший artifact:

1. artifact совместим по min_os;
2. artifact не заблокирован по max_supported_os;
3. artifact подходит по архитектуре;
4. artifact подходит по битности;
5. artifact имеет максимальную версию приложения среди совместимых;
6. если есть несколько artifact для одной версии, выбрать наиболее подходящий;
7. если совместимых нет, вернуть лучший несовместимый вариант с причинами.

### 11.8. max_supported_os и max_tested_os

Нужно различать:

```text
max_supported_os
max_tested_os
hard_block_above_max
```

Если `hard_block_above_max = true` и пользовательская ОС выше `max_supported_os`, artifact блокируется.

Если `hard_block_above_max = false`, но ОС выше `max_tested_os`, статус может быть:

```text
untested
probably_compatible
```

---

## 12. API endpoints

### 12.1. Public/catalog API

```http
GET /api/v1/bootstrap
GET /api/v1/categories
GET /api/v1/apps
GET /api/v1/apps/{slug}
GET /api/v1/apps/{slug}/versions
GET /api/v1/apps/{slug}/reviews
GET /api/v1/search
GET /api/v1/download/{artifact_id}
```

### 12.2. Auth API

```http
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/logout
POST /api/v1/auth/refresh
POST /api/v1/auth/2fa/setup
POST /api/v1/auth/2fa/verify
POST /api/v1/auth/oauth/google
POST /api/v1/auth/oauth/apple
POST /api/v1/auth/legacy/login
```

### 12.3. User API

```http
GET /api/v1/me
PATCH /api/v1/me
POST /api/v1/me/avatar
POST /api/v1/me/api-tokens
POST /api/v1/me/legacy-passwords
DELETE /api/v1/me/legacy-passwords/{id}
```

### 12.4. Reviews API

```http
POST /api/v1/apps/{slug}/reviews
PATCH /api/v1/reviews/{id}
DELETE /api/v1/reviews/{id}
POST /api/v1/reviews/{id}/like
DELETE /api/v1/reviews/{id}/like
POST /api/v1/reviews/{id}/replies
```

### 12.5. Admin/moderation API

```http
GET /api/v1/admin/dashboard
GET /api/v1/admin/users
PATCH /api/v1/admin/users/{id}
POST /api/v1/admin/users/{id}/roles
DELETE /api/v1/admin/users/{id}/roles/{role}

GET /api/v1/admin/apps
POST /api/v1/admin/apps
PATCH /api/v1/admin/apps/{id}
DELETE /api/v1/admin/apps/{id}

POST /api/v1/admin/apps/{id}/versions
PATCH /api/v1/admin/versions/{id}
DELETE /api/v1/admin/versions/{id}

POST /api/v1/admin/versions/{id}/artifacts
PATCH /api/v1/admin/artifacts/{id}
DELETE /api/v1/admin/artifacts/{id}

POST /api/v1/admin/artifacts/{id}/mirrors
POST /api/v1/admin/screenshots
POST /api/v1/admin/icons

GET /api/v1/admin/moderation
POST /api/v1/admin/moderation/{id}/approve
POST /api/v1/admin/moderation/{id}/reject

GET /api/v1/admin/audit-log
```

---

## 13. Примеры API

### 13.1. Bootstrap

Request:

```http
GET /api/v1/bootstrap
```

Response:

```json
{
  "api_version": "v1",
  "catalog_format_version": 1,
  "server_status": "ok",
  "features": {
    "reviews": true,
    "legacy_auth": true,
    "p2p": false,
    "signed_catalog": false
  }
}
```

### 13.2. Список приложений

Request:

```http
GET /api/v1/apps?os=10.9.5&arch=x86_64&limit=24&page=1&category=graphics-design
```

Response:

```json
{
  "page": 1,
  "limit": 24,
  "apps": [
    {
      "slug": "pixelmator",
      "name": "Pixelmator",
      "category": "Graphics & Design",
      "summary": "Image editing for the creative mind.",
      "icon": "https://cdn.example.local/icons/pixelmator/3.6.png",
      "rating": 4.7,
      "rating_count": 256,
      "recommended_version": "3.6",
      "arch_badges": ["Intel"],
      "compatibility": {
        "status": "compatible",
        "label": "Совместимо"
      }
    }
  ]
}
```

### 13.3. Страница приложения

Request:

```http
GET /api/v1/apps/pixelmator?os=10.9.5&arch=x86_64
```

Response:

```json
{
  "slug": "pixelmator",
  "name": "Pixelmator",
  "developer_name": "Pixelmator Team",
  "summary": "Image editor for Mac.",
  "description": "Full app description.",
  "category": "Graphics & Design",
  "recommended_artifact": {
    "id": "pixelmator-3.6-dmg",
    "version": "3.6",
    "file_name": "Pixelmator_3.6.dmg",
    "package_type": "dmg",
    "size_bytes": 205940326,
    "sha256": "expected_sha256_here",
    "min_os": "10.8",
    "max_supported_os": "10.13",
    "archs": ["x86_64"]
  },
  "compatibility": {
    "status": "compatible",
    "level": "recommended",
    "reasons": []
  }
}
```

### 13.4. Несовместимость

Response:

```json
{
  "slug": "modern-app",
  "name": "Modern App",
  "compatibility": {
    "status": "blocked",
    "level": "blocked",
    "reasons": [
      {
        "code": "os_too_old",
        "message": "Версия системы 10.9 ниже минимальной требуемой продуктом 10.15."
      }
    ]
  }
}
```

---

## 14. Авторизация и безопасность

### 14.1. Главный принцип

Legacy-клиент не должен принимать и передавать основной пароль аккаунта.

Основной пароль аккаунта используется только в современной web UI.

### 14.2. Legacy-клиент

Legacy-клиент использует:

```text
email
app-specific legacy password
```

App-specific password создаётся только в web UI.

После создания пароль показывается пользователю один раз.

Legacy-клиент отправляет email и app-specific password только по HTTPS.

Если TLS не работает, авторизация запрещена.

### 14.3. Запрещено в legacy-клиенте

```text
основной пароль аккаунта
OAuth login
загрузка файлов
модерация
админские функции
управление ролями
смена e-mail
смена основного пароля
отключение 2FA
```

### 14.4. Разрешённые scope для legacy-клиента

```text
catalog:read
downloads:read
reviews:write
reviews:like
profile:read_basic
```

### 14.5. Запрещённые scope для legacy-клиента

```text
apps:upload
apps:edit
artifacts:upload
artifacts:edit
moderation:approve
moderation:reject
users:manage
roles:manage
admin:any
profile:change_email
profile:change_password
profile:disable_2fa
```

### 14.6. TLS policy

Для всех авторизованных endpoint:

```text
HTTPS only
```

Если происходит одна из ошибок:

```text
TLS handshake failed
certificate expired
hostname mismatch
unknown CA
pinning failed
```

то клиент должен:

- заблокировать авторизацию;
- не показывать кнопку `Продолжить всё равно`;
- разрешить только offline/cache режим.

### 14.7. Публичный read-only fallback

HTTP fallback допустим только для:

- публичного каталога;
- публичных файлов;
- картинок;
- неавторизованных данных.

При этом каталог должен быть подписан.

HTTP fallback запрещён для:

- login;
- token refresh;
- reviews;
- likes;
- profile;
- upload;
- moderation;
- admin API.

### 14.8. Подпись каталога

В будущем каталог должен подписываться сервером.

Клиент должен хранить публичный ключ.

Если подпись каталога не проходит:

- новый каталог отвергается;
- используется последний валидный cache;
- пользователь получает ошибку.

---

## 15. Web UI

### 15.1. Назначение

Web UI используется для:

- регистрации;
- входа;
- OAuth через Google/Apple;
- настройки профиля;
- настройки 2FA;
- создания app-specific passwords;
- загрузки файлов;
- модерации;
- администрирования;
- просмотра каталога из браузера.

### 15.2. Стиль

Web UI должен визуально напоминать LegacyStore native client.

Стиль:

```text
OS X Mavericks
classic Mac App Store-inspired
light gray toolbar
left sidebar
content grid
app detail page
rounded panels
```

Не использовать fake native window frame.

### 15.3. Основные разделы web UI

```text
Home
Catalog
App Page
Login
Register
Profile
Security
Legacy Passwords
Uploads
Moderation Queue
Admin Dashboard
Users
Apps
Versions
Artifacts
Reviews
Audit Log
Settings
```

### 15.4. Upload через web UI

Загрузка файлов разрешена только ролям:

```text
trusted
moder
admin
```

Загруженный файл не публикуется сразу.

Pipeline:

```text
1. User uploads file.
2. File goes to quarantine.
3. Backend calculates SHA-256.
4. Metadata extractor inspects file.
5. Backend creates pending artifact.
6. Moderator approves or rejects.
7. Approved artifact moves to public storage.
8. API starts exposing it.
```

### 15.5. Безопасность web UI

Web UI должен использовать:

- HTTPS;
- secure cookies;
- SameSite cookies;
- CSRF protection;
- rate limiting;
- CSP headers;
- password hashing;
- 2FA TOTP;
- recovery codes;
- audit log.

---

## 16. File storage

### 16.1. Storage backends

Первая версия:

```text
local filesystem
```

Будущие backend:

```text
S3-compatible storage
CDN
external mirrors
```

### 16.2. Физическая структура storage

Пример:

```text
/storage/
├── apps/
│   ├── graphics-design/
│   │   └── pixelmator/
│   │       ├── 3.6/
│   │       │   ├── Pixelmator_3.6.dmg
│   │       │   ├── Pixelmator_3.6.dmg.torrent
│   │       │   ├── icon_512.png
│   │       │   └── screenshots/
│   │       └── 3.5.8/
│   └── internet-network/
│       └── vlc/
├── icons/
├── screenshots/
├── torrents/
└── quarantine/
```

База данных остаётся главным источником истины.

Файловая структура нужна только для физического хранения.

### 16.3. External links

Artifact может иметь:

- локальный файл;
- внешнюю прямую ссылку;
- ссылку на страницу загрузки;
- зеркало;
- web seed;
- torrent;
- magnet.

---

## 17. Metadata extractor

### 17.1. Назначение

Инструмент должен извлекать метаданные из файлов приложений.

Входные типы:

```text
.app
.dmg
.zip
.pkg
```

### 17.2. Для `.app`

Извлекать:

```text
CFBundleIdentifier
CFBundleName
CFBundleDisplayName
CFBundleShortVersionString
CFBundleVersion
LSMinimumSystemVersion
architectures
SHA-256
```

Архитектуры определять через:

```text
lipo
file
Mach-O parser
```

### 17.3. Для `.dmg`

Допустимая логика:

```text
1. Смонтировать во временную директорию.
2. Найти .app.
3. Извлечь Info.plist.
4. Определить архитектуры.
5. Размонтировать.
```

### 17.4. Выходной JSON

```json
{
  "bundle_id": "org.example.app",
  "name": "Example App",
  "version": "1.2.3",
  "minimum_os": "10.7",
  "architectures": ["i386", "x86_64"],
  "package_type": "dmg",
  "sha256": "hash_here",
  "size_bytes": 123456789
}
```

---

## 18. Docker Compose

### 18.1. Обязательные сервисы

```text
postgres
backend
admin-web
```

### 18.2. postgres

Требования:

- официальный образ PostgreSQL;
- persistent volume;
- environment из `.env`;
- healthcheck.

### 18.3. backend

Требования:

- build из `./backend`;
- зависит от healthcheck PostgreSQL;
- порт по умолчанию `8080`;
- читает `.env`.

### 18.4. admin-web

Возможные варианты:

- отдельный сервис;
- статические assets, отдаваемые backend;
- Go templates внутри backend.

Если отдельный сервис:

- порт по умолчанию `8081`;
- читает `.env`.

### 18.5. reverse-proxy

Опционально:

```text
Caddy
Nginx
```

Локальный запуск не должен зависеть от reverse-proxy.

---

## 19. Скрипты

### 19.1. scripts/build_backend.sh

Должен:

- запускать `gofmt`;
- запускать `go test`;
- собирать backend;
- складывать бинарь в `./build/legacystore-backend`;
- завершаться с ошибкой при неуспехе.

### 19.2. scripts/build_web.sh

Должен:

- подготовить web UI;
- скопировать static assets, если нужно;
- если отдельная сборка не требуется, явно написать это;
- завершаться с ошибкой при неуспехе.

### 19.3. scripts/build_client.sh

Должен:

- проверять наличие `xcodebuild`;
- определять текущую macOS;
- использовать переменные:

```text
LEGACYSTORE_CLIENT_SCHEME
LEGACYSTORE_CLIENT_CONFIGURATION
LEGACYSTORE_CLIENT_SDK
LEGACYSTORE_CLIENT_ARCHS
LEGACYSTORE_CLIENT_DEPLOYMENT_TARGET
LEGACYSTORE_CLIENT_DERIVED_DATA
```

Значения по умолчанию:

```text
configuration = Release
archs = i386 x86_64
deployment target = 10.5
```

Если toolchain не поддерживает сборку, вывести понятную ошибку.

### 19.4. scripts/dev_up.sh

Должен:

- запускать Docker Compose;
- читать `.env`;
- выводить URL backend и web UI.

### 19.5. scripts/dev_down.sh

Должен:

- останавливать Docker Compose.

### 19.6. scripts/migrate.sh

Должен:

- запускать миграции PostgreSQL;
- работать внутри Docker Compose окружения;
- понятно падать, если база недоступна.

### 19.7. scripts/seed.sh

Должен:

- загружать тестовые категории;
- загружать минимум 10 тестовых приложений;
- загружать версии и artifacts.

### 19.8. scripts/test_api.sh

Скрипт должен быть полностью на английском языке.

Требования:

- POSIX shell;
- curl;
- jq;
- `BASE_URL` по умолчанию `http://localhost:8080`;
- возможность переопределить:

```sh
BASE_URL=http://localhost:8080 ./scripts/test_api.sh
```

Вывод:

```text
<TestName>: [OK]
<TestName>: [NORMAL]
<TestName>: [ERR]
```

Цвета:

- `[OK]` зелёным;
- `[NORMAL]` жёлтым;
- `[ERR]` красным.

Если тест упал:

```text
<TestName> caught error <ErrorName>:
> Expected: <x>
> Got: <y>
```

В конце:

```text
All API tests passed.
Summary:
- Passed: <n>
- Normal: <n>
- Failed: 0
```

или:

```text
API tests finished with errors.
Summary:
- Passed: <n>
- Normal: <n>
- Failed: <n>
```

Не сравнивать JSON побитово.

Игнорировать volatile fields:

```text
server_version
build_date
generated_at
request_id
uptime
timestamps
```

---

## 20. API tests

Минимальные тесты:

### 20.1. Bootstrap

```http
GET /api/v1/bootstrap
```

Проверить:

- HTTP 200;
- есть `catalog_format_version`;
- есть `api_version`;
- есть `server_status`;
- `server_status = ok`.

### 20.2. Categories

```http
GET /api/v1/categories
```

Проверить:

- HTTP 200;
- есть массив `categories`;
- массив не пустой;
- у категории есть `slug` и `name`.

### 20.3. App list

```http
GET /api/v1/apps?os=10.9.5&arch=x86_64&limit=12&page=1
```

Проверить:

- HTTP 200;
- есть массив `apps`;
- у app есть `slug`, `name`, `compatibility`.

### 20.4. Search

```http
GET /api/v1/search?q=vlc&os=10.9.5&arch=x86_64
```

Проверить:

- HTTP 200;
- есть массив `results`.

Если fixture содержит VLC, ожидать VLC.

### 20.5. App detail

```http
GET /api/v1/apps/pixelmator?os=10.9.5&arch=x86_64
```

Проверить:

- HTTP 200;
- есть `slug`;
- есть `name`;
- есть `versions` или `recommended_artifact`;
- есть `compatibility`.

### 20.6. Compatibility: OS too old

```http
GET /api/v1/apps/pixelmator?os=10.5.8&arch=i386
```

Проверить:

- HTTP 200;
- есть `compatibility`;
- если blocked, есть reasons.

### 20.7. Compatibility: macOS 10.15 and 32-bit

```http
GET /api/v1/apps/legacy-32bit-test?os=10.15&arch=x86_64
```

Проверить:

- HTTP 200;
- status blocked или incompatible;
- reason содержит `bitness_mismatch` или `requires_32bit`.

### 20.8. Download metadata

```http
GET /api/v1/download/{artifact_id}
```

Проверить:

- HTTP 200;
- есть download URL или source info;
- есть `sha256`;
- есть `size_bytes`.

### 20.9. Invalid login

```http
POST /api/v1/auth/login
```

Body:

```json
{
  "email": "invalid@example.com",
  "password": "wrong"
}
```

Проверить:

- HTTP 401 или 403;
- token не возвращается.

### 20.10. Protected endpoint without token

```http
GET /api/v1/me
```

Проверить:

- HTTP 401;
- профиль не возвращается.

### 20.11. Reviews list

```http
GET /api/v1/apps/pixelmator/reviews
```

Проверить:

- HTTP 200;
- есть массив `reviews`.

### 20.12. Unknown app

```http
GET /api/v1/apps/this-app-does-not-exist
```

Проверить:

- HTTP 404.

---

## 21. Makefile

Добавить команды:

```makefile
dev-up
dev-down
backend
web
client
migrate
seed
test-api
test-backend
clean
```

Описание:

| Команда | Назначение |
|---|---|
| `make dev-up` | Запустить Docker окружение |
| `make dev-down` | Остановить Docker окружение |
| `make backend` | Собрать backend |
| `make web` | Собрать web UI |
| `make client` | Собрать macOS клиент |
| `make migrate` | Запустить миграции |
| `make seed` | Загрузить тестовые данные |
| `make test-api` | Запустить curl API tests |
| `make test-backend` | Запустить backend unit tests |
| `make clean` | Удалить build artifacts |

---

## 22. .env.example

Создать `.env.example`:

```env
POSTGRES_DB=legacystore
POSTGRES_USER=legacystore
POSTGRES_PASSWORD=legacystore_dev_password
DATABASE_URL=postgres://legacystore:legacystore_dev_password@postgres:5432/legacystore?sslmode=disable

BACKEND_HOST=0.0.0.0
BACKEND_PORT=8080
PUBLIC_BASE_URL=http://localhost:8080
ADMIN_WEB_BASE_URL=http://localhost:8081

JWT_SECRET=change_me_in_production
SESSION_SECRET=change_me_in_production

STORAGE_BACKEND=local
LOCAL_STORAGE_PATH=/data/storage

CATALOG_SIGNING_ENABLED=false
CATALOG_PUBLIC_KEY_PATH=
CATALOG_PRIVATE_KEY_PATH=

OAUTH_GOOGLE_CLIENT_ID=
OAUTH_GOOGLE_CLIENT_SECRET=
OAUTH_APPLE_CLIENT_ID=
OAUTH_APPLE_TEAM_ID=
OAUTH_APPLE_KEY_ID=
OAUTH_APPLE_PRIVATE_KEY_PATH=
```

---

## 23. README.md

README должен быть на русском языке.

Стиль:

- простой;
- понятный;
- без шуток;
- без эмодзи;
- почти как документация, но не чрезмерно строго.

README должен содержать разделы:

1. Краткая информация;
2. Возможности;
3. Поддерживаемые системы;
4. Установка зависимостей и тулчейна;
5. Настройка окружения;
6. Запуск backend и web UI через Docker Compose;
7. Миграции и тестовые данные;
8. Сборка клиента;
9. Сборка backend и web UI;
10. Тестирование;
11. Основные команды Makefile;
12. Безопасность;
13. Статус проекта.

В начале README должен быть небольшой скриншот:

```md
![LegacyStore](docs/images/legacystore-main.png)
```

Если файла нет, создать директорию:

```text
docs/images/
```

и оставить placeholder.

---

## 24. AGENTS.md

Создать `AGENTS.md` с правилами для Codex/AI agents.

Минимальное содержимое:

```md
# LegacyStore Agent Instructions

## Hard Requirements

- Do not use Node.js for backend.
- Backend must be written in Go.
- Database must be PostgreSQL.
- Native legacy client must be written in Objective-C and AppKit.
- Do not use Swift in the legacy client.
- Do not use SwiftUI.
- Do not use Storyboards.
- Do not use ARC in the legacy client.
- Do not use Auto Layout as a hard dependency.
- Target native client: Mac OS X 10.5 through macOS 10.15 Intel.
- Target architectures: i386 and x86_64.
- PPC is out of scope.
- Do not silently drop i386.
- Do not silently increase deployment target.
- Keep API responses compact.
- Use pagination everywhere.
- Do not implement software uploads in the legacy client.
- Uploads are allowed only through the HTTPS web interface.
- Legacy client must never send the primary account password.
- Legacy client may use only app-specific passwords.
- No authentication is allowed without working HTTPS/TLS.
- If TLS validation fails, authentication must be blocked.
- Hash mismatch must not delete the downloaded file automatically.
- Downloaded files must never auto-open after download.

## UI

Use the provided UI concept images as reference.

The native client should visually resemble the classic Mac App Store / OS X Mavericks-era AppKit interface.

The web UI should visually resemble the native app, but without a fake native window frame.

If UI concept images are missing, ask for them instead of inventing the interface.

## Development Style

- Keep code simple.
- Avoid large dependencies without justification.
- Do not make arbitrary product decisions.
- Do not change the architecture without explicit approval.
- Add clear TODO comments where old macOS toolchain-specific work is required.
```

---

## 25. Первая итерация разработки

Первая итерация не должна пытаться реализовать весь продукт.

### 25.1. Реализовать в v0.1

```text
1. Структура репозитория.
2. AGENTS.md.
3. README.md.
4. docs/PROJECT_SPEC.md.
5. docs/SECURITY.md.
6. docker-compose.yml.
7. .env.example.
8. Makefile.
9. Go backend skeleton.
10. PostgreSQL migrations.
11. Seed data.
12. Basic REST API:
    - /api/v1/bootstrap
    - /api/v1/categories
    - /api/v1/apps
    - /api/v1/apps/{slug}
    - /api/v1/search
13. Compatibility engine.
14. Unit tests for compatibility.
15. scripts/test_api.sh.
16. Objective-C client skeleton.
17. Admin web UI skeleton.
```

### 25.2. Не реализовывать в v0.1

```text
Real P2P downloader
Real OAuth
Real file uploads
Real CDN
Full moderation panel
Full native client UI
Payment system
PPC support
Mac OS Classic support
Apple Silicon support
```

---

## 26. Правила для Codex

При работе над проектом Codex должен:

1. Сначала читать:
   - `AGENTS.md`;
   - `docs/PROJECT_SPEC.md`;
   - `docs/TASKS.md`;
   - `docs/SECURITY.md`.

2. Реализовывать только текущую задачу из `docs/TASKS.md`.

3. Не реализовывать функции, помеченные как `Do not implement yet`.

4. Не менять архитектуру без явного запроса.

5. Не добавлять Node.js backend.

6. Не заменять Objective-C клиент на Swift/Electron/WebView.

7. Не игнорировать требования к 10.5/i386.

8. Не заявлять, что сборка под старые macOS проверена, если она не была реально проверена.

9. Не удалять файл при hash mismatch.

10. Не включать автозапуск скачанных файлов.

11. Не добавлять авторизацию без HTTPS/TLS.

---

## 27. Ожидаемая структура репозитория

```text
LegacyStore/
├── AGENTS.md
├── README.md
├── LICENSE
├── Makefile
├── .env.example
├── docker-compose.yml
├── backend/
│   ├── cmd/
│   ├── internal/
│   ├── migrations/
│   ├── fixtures/
│   ├── go.mod
│   └── go.sum
├── admin-web/
│   ├── templates/
│   ├── static/
│   └── README.md
├── legacy-client/
│   ├── LegacyStore.xcodeproj
│   ├── LegacyStore/
│   └── README.md
├── tools/
│   └── metadata-extractor/
├── scripts/
│   ├── build_backend.sh
│   ├── build_web.sh
│   ├── build_client.sh
│   ├── dev_up.sh
│   ├── dev_down.sh
│   ├── migrate.sh
│   ├── seed.sh
│   └── test_api.sh
├── docs/
│   ├── PROJECT_SPEC.md
│   ├── TASKS.md
│   ├── SECURITY.md
│   ├── development.md
│   └── images/
│       └── legacystore-main.png
├── fixtures/
└── docker/
```

---

## 28. Критерии готовности MVP

MVP считается готовым, если:

1. Docker Compose поднимает PostgreSQL, backend и web UI.
2. `/api/v1/bootstrap` возвращает статус сервера.
3. `/api/v1/categories` возвращает категории.
4. `/api/v1/apps` возвращает список приложений с compatibility status.
5. `/api/v1/apps/{slug}` возвращает карточку приложения.
6. Compatibility engine корректно обрабатывает:
   - слишком старую macOS;
   - слишком новую macOS;
   - mismatch архитектуры;
   - 32-bit-only app на macOS 10.15;
   - untested newer OS.
7. `scripts/test_api.sh` проходит тесты.
8. Есть тестовые данные минимум для 10 приложений.
9. Есть skeleton нативного клиента.
10. Есть skeleton web UI.
11. README объясняет запуск и сборку.
12. Security model задокументирован.

---

## 29. Лицензия

Код проекта распространяется под:

```text
AGPL-3.0-or-later
```

Важно:

Лицензия проекта относится только к исходному коду LegacyStore.

Загружаемые через каталог сторонние приложения, иконки, скриншоты, описания и метаданные могут иметь собственные лицензии и условия распространения.

LegacyStore не должен автоматически считать сторонний софт свободным для зеркалирования.

Для каждого artifact нужно хранить:

```text
license_type
source_url
mirror_allowed
source_notes
```

---

## 30. Финальное замечание

LegacyStore должен развиваться поэтапно.

Главный приоритет первой версии:

```text
каталог → compatibility engine → HTTP download → SHA-256 verification → web admin skeleton
```

P2P, OAuth, CDN, полноценная модерация и расширенная web UI должны добавляться после появления стабильного ядра проекта.
