# CJM — Клиент (Пользователь Food Delivery API)

## Journey Map (PlantUML Activity Diagram)

```plantuml
@startuml CJM_Client
title CJM: Клиентский путь — Заказ еды на Food Delivery API

|Клиент|
start
:Открывает раздел "Food Delivery API";
:Просматривает витрину ресторанов;

if (Ресторан открыт?) then (Да)
    :Переходит на страницу ресторана;
    :Изучает меню по категориям;
    :Добавляет позиции в корзину;
else (Нет)
    :Видит плашку "Временно недоступно";
    stop
endif

:Переходит к оформлению;

|Food Delivery API Core|
:Валидация корзины:\nпроверка наличия (стоп-лист)\nи актуальности цен;

if (Всё доступно и цены совпадают?) then (Да)
    |Клиент|
    :Подтверждает заказ, адрес и телефон;
    |Food Delivery API Core|
    :Создаёт заказ (статус CREATED);
    :Отправляет webhook в ресторан;
else (Нет)
    |Клиент|
    :Получает 409: "Блюдо X закончилось"\nили "Цена изменилась";
    :Корректирует корзину;
    detach
endif

|Клиент|
:Экран ожидания подтверждения;

|Ресторан|
if (Ресторан принял заказ в течение 10 мин?) then (Да)
    :Статус → CONFIRMED;
    |Клиент|
    :Видит "Заказ готовится";
    |Ресторан|
    :Статус → COOKING;
    :Статус → READY_FOR_PICKUP;
    |Food Delivery API Core|
    :Статус → IN_DELIVERY;
    :Статус → DELIVERED;
    |Клиент|
    :Получает заказ;
    stop
else (Нет / Отклонён)
    |Food Delivery API Core|
    :Статус → CANCELLED\n(причина: таймаут или отклонение);
    |Клиент|
    :Уведомление об отмене;
    stop
endif
@enduml
```

---

## Сценарии (Sequence Diagrams)

### Успешный заказ

```mermaid
sequenceDiagram
    participant C as Клиент
    participant API as Core API
    participant R as Ресторан

    C->>API: GET /restaurants
    API-->>C: Список ресторанов

    C->>API: GET /restaurants/{id}/menu
    API-->>C: Меню с категориями

    C->>API: POST /orders (X-User-ID)
    API->>R: Webhook: новый заказ (HMAC)
    API-->>C: 201 Created (status: CREATED)

    R->>API: POST /partner/orders/{id}/accept
    API-->>R: 200 OK (status: CONFIRMED)

    R->>API: PATCH /partner/orders/{id}/status (COOKING)
    R->>API: PATCH /partner/orders/{id}/status (READY_FOR_PICKUP)

    Note over API: Курьерская служба
    API->>API: POST /internal/.../delivery-step (IN_DELIVERY)
    API->>API: POST /internal/.../delivery-step (DELIVERED)

    C->>API: GET /orders/{id}
    API-->>C: status: DELIVERED
```

### Блюдо в стоп-листе (409 Conflict)

```mermaid
sequenceDiagram
    participant C as Клиент
    participant API as Core API

    C->>API: POST /orders (недоступное блюдо)
    API-->>C: 409 Conflict
    Note right of C: unavailable_items[]<br/>price_discrepancies[]

    C->>C: Убирает проблемные блюда
    C->>API: POST /orders (исправленная корзина)
    API-->>C: 201 Created
```

### Отмена клиентом

```mermaid
sequenceDiagram
    participant C as Клиент
    participant API as Core API

    C->>API: POST /orders
    API-->>C: 201 (CREATED)

    C->>API: POST /orders/{id}/cancel
    API-->>C: 200 (CANCELLED, reason: CANCELLED_BY_USER)
```

### Таймаут ресторана

```mermaid
sequenceDiagram
    participant C as Клиент
    participant API as Core API
    participant W as TimeoutWorker
    participant R as Ресторан

    C->>API: POST /orders
    API->>R: Webhook (ресторан не отвечает)
    API-->>C: 201 (CREATED)

    Note over W: Через 10 минут
    W->>API: Проверяет просроченные заказы
    W->>API: Отменяет (TIMEOUT_NO_RESTAURANT_REPLY)

    C->>API: GET /orders/{id}
    API-->>C: status: CANCELLED
```
