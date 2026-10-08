# CJM — Ресторан-партнёр (Food Delivery API)

## Journey Map (PlantUML Activity Diagram)

```plantuml
@startuml CJM_Restaurant
title CJM: Путь ресторана-партнёра на Food Delivery API

|Food Delivery API Core|
:Клиент оформляет заказ;
:Отправка webhook в ресторан\n(с HMAC-подписью X-Signature);

|Ресторан (POS)|
:Получает webhook;
:Верифицирует подпись;

if (Подпись корректна?) then (Да)
    :Отвечает 200 OK;
else (Нет)
    :Отвечает 401 — отклоняет;
    stop
endif

:Проверяет наличие ингредиентов\nи загрузку кухни;

if (Может выполнить заказ?) then (Да)
    :POST /partner/orders/{id}/accept\n(указывает время готовки);
    |Food Delivery API Core|
    :Статус → CONFIRMED;
    |Ресторан (POS)|
    :Начинает готовить;
    :PATCH /partner/orders/{id}/status → COOKING;
    :Блюдо готово;
    :PATCH /partner/orders/{id}/status → READY_FOR_PICKUP;
    :Ожидает курьера;
    stop
else (Нет — перегрузка / нет ингредиентов)
    :POST /partner/orders/{id}/reject;
    |Food Delivery API Core|
    :Статус → CANCELLED\n(REJECTED_BY_RESTAURANT);
    stop
endif

@enduml
```

### Управление стоп-листом

```plantuml
@startuml Restaurant_StopList
title Управление стоп-листом

|Ресторан|
:Закончился ингредиент;
:PATCH /partner/menu/items/{id}\n(is_available: false);

|Food Delivery API Core|
:Блюдо помечено как недоступное;

|Клиент|
:Видит блюдо как "Недоступно" в меню;
if (Пытается заказать?) then (Да)
    |Food Delivery API Core|
    :Возвращает 409 Conflict\n(unavailable_items);
else (Нет)
    :Выбирает другое блюдо;
endif

|Ресторан|
:Ингредиент снова в наличии;
:PATCH /partner/menu/items/{id}\n(is_available: true);
|Food Delivery API Core|
:Блюдо снова доступно;
stop

@enduml
```

---

## Сценарии (Sequence Diagrams)

### Приём и выполнение заказа

```mermaid
sequenceDiagram
    participant API as Core API
    participant R as Ресторан (POS)

    API->>R: POST /webhooks/orders/incoming (HMAC)
    Note right of R: Верифицирует X-Signature
    R-->>API: 200 OK

    R->>API: POST /partner/orders/{id}/accept (25 мин)
    API-->>R: 200 (CONFIRMED)

    R->>API: PATCH /partner/orders/{id}/status (COOKING)
    API-->>R: 200

    R->>API: PATCH /partner/orders/{id}/status (READY_FOR_PICKUP)
    API-->>R: 200

    Note over R: Курьер забирает заказ
```

### Отклонение заказа

```mermaid
sequenceDiagram
    participant API as Core API
    participant R as Ресторан

    API->>R: POST /webhooks/orders/incoming
    R-->>API: 200 OK

    Note right of R: Нет ингредиентов
    R->>API: POST /partner/orders/{id}/reject
    API-->>R: 200 (CANCELLED)
```

### Стоп-лист

```mermaid
sequenceDiagram
    participant R as Ресторан
    participant API as Core API
    participant C as Клиент

    R->>API: PATCH /partner/menu/items/{id} (is_available: false)
    API-->>R: 200

    C->>API: POST /orders (содержит недоступное блюдо)
    API-->>C: 409 Conflict (unavailable_items)

    R->>API: PATCH /partner/menu/items/{id} (is_available: true)
    API-->>R: 200
```

### Автоматический pipeline (mock-ресторан)

При `RESTAURANT_AUTO_ACCEPT=true` мок-ресторан автоматически проводит заказ через весь цикл:

```mermaid
sequenceDiagram
    participant API as Core API
    participant M as Restaurant Mock

    API->>M: Webhook: новый заказ
    M-->>API: 200 OK

    Note over M: Авто-режим
    M->>API: AcceptOrder (25 мин)
    Note over M: Пауза 2 сек
    M->>API: UpdateStatus → COOKING
    Note over M: Пауза 2 сек
    M->>API: UpdateStatus → READY_FOR_PICKUP
```
