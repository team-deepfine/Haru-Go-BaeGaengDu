# Haru API Documentation

Base URL: `http://localhost:8080`

---

## Health Check

```
GET /health
```

**Response** `200 OK`

```json
{
  "status": "ok"
}
```

---

## Event API

### 1. 일정 생성

```
POST /api/events
```

**Request Body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| title | string | O | 일정 제목 |
| startAt | string | O | 시작 시간 (ISO-8601, e.g. `2024-03-10T09:00:00Z`) |
| endAt | string | O | 종료 시간 (ISO-8601, e.g. `2024-03-10T10:00:00Z`) |
| allDay | boolean | | 종일 여부 (default: `false`) |
| timezone | string | | IANA 타임존 (default: `"UTC"`, e.g. `"Asia/Seoul"`) |
| locationName | string \| null | | 장소명 |
| locationAddress | string \| null | | 주소 |
| locationLat | number \| null | | 위도 |
| locationLng | number \| null | | 경도 |
| reminderOffsets | number[] | | 알림 오프셋 배열 (default: `[]`) |
| notes | string \| null | | 메모 |

**Example**

```bash
curl -X POST http://localhost:8080/api/events \
  -H "Content-Type: application/json" \
  -d '{
    "title": "팀 회의",
    "startAt": "2024-03-10T09:00:00Z",
    "endAt": "2024-03-10T10:00:00Z",
    "timezone": "Asia/Seoul",
    "locationName": "회의실 A",
    "reminderOffsets": [300, 900]
  }'
```

**Response** `201 Created`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "팀 회의",
  "startAt": "2024-03-10T09:00:00Z",
  "endAt": "2024-03-10T10:00:00Z",
  "allDay": false,
  "timezone": "Asia/Seoul",
  "locationName": "회의실 A",
  "reminderOffsets": [300, 900],
  "createdAt": "2024-03-06T12:00:00Z",
  "updatedAt": "2024-03-06T12:00:00Z"
}
```

> **Note:** `locationName`, `locationAddress`, `locationLat`, `locationLng`, `notes` 필드는 값이 없으면 응답에 포함되지 않습니다 (`omitempty`).

---

### 2. 일정 단건 조회

```
GET /api/events/:id
```

**Path Parameters**

| Parameter | Type | Description |
|-----------|------|-------------|
| id | UUID | 일정 ID |

**Example**

```bash
curl http://localhost:8080/api/events/550e8400-e29b-41d4-a716-446655440000
```

**Response** `200 OK`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "팀 회의",
  "startAt": "2024-03-10T09:00:00Z",
  "endAt": "2024-03-10T10:00:00Z",
  "allDay": false,
  "timezone": "Asia/Seoul",
  "locationName": "회의실 A",
  "reminderOffsets": [300, 900],
  "createdAt": "2024-03-06T12:00:00Z",
  "updatedAt": "2024-03-06T12:00:00Z"
}
```

---

### 3. 일정 목록 조회 (날짜 범위)

```
GET /api/events?start={start}&end={end}
```

**Query Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| start | string | O | 조회 시작 시간 (ISO-8601) |
| end | string | O | 조회 종료 시간 (ISO-8601) |

**Example**

```bash
curl "http://localhost:8080/api/events?start=2024-03-01T00:00:00Z&end=2024-03-31T23:59:59Z"
```

**Response** `200 OK`

```json
{
  "events": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "title": "팀 회의",
      "startAt": "2024-03-10T09:00:00Z",
      "endAt": "2024-03-10T10:00:00Z",
      "allDay": false,
      "timezone": "Asia/Seoul",
      "reminderOffsets": [],
      "createdAt": "2024-03-06T12:00:00Z",
      "updatedAt": "2024-03-06T12:00:00Z"
    }
  ],
  "count": 1
}
```

---

### 4. 일정 수정

```
PUT /api/events/:id
```

**Path Parameters**

| Parameter | Type | Description |
|-----------|------|-------------|
| id | UUID | 일정 ID |

**Request Body**

일정 생성과 동일한 필드 구조.

**Example**

```bash
curl -X PUT http://localhost:8080/api/events/550e8400-e29b-41d4-a716-446655440000 \
  -H "Content-Type: application/json" \
  -d '{
    "title": "팀 회의 (변경)",
    "startAt": "2024-03-10T10:00:00Z",
    "endAt": "2024-03-10T11:00:00Z",
    "timezone": "Asia/Seoul"
  }'
```

**Response** `200 OK`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "팀 회의 (변경)",
  "startAt": "2024-03-10T10:00:00Z",
  "endAt": "2024-03-10T11:00:00Z",
  "allDay": false,
  "timezone": "Asia/Seoul",
  "reminderOffsets": [],
  "createdAt": "2024-03-06T12:00:00Z",
  "updatedAt": "2024-03-06T12:30:00Z"
}
```

---

### 5. 일정 삭제

```
DELETE /api/events/:id
```

**Path Parameters**

| Parameter | Type | Description |
|-----------|------|-------------|
| id | UUID | 일정 ID |

**Example**

```bash
curl -X DELETE http://localhost:8080/api/events/550e8400-e29b-41d4-a716-446655440000
```

**Response** `204 No Content`

(응답 본문 없음)

---

## Error Response

모든 에러는 [RFC 7807 Problem Details](https://tools.ietf.org/html/rfc7807) 형식을 따릅니다.

```json
{
  "type": "about:blank",
  "title": "Bad Request",
  "status": 400,
  "detail": "title is required"
}
```

**Validation Error** (필드별 에러가 있는 경우)

```json
{
  "type": "about:blank",
  "title": "Bad Request",
  "status": 400,
  "detail": "Validation failed",
  "errors": [
    { "field": "startAt", "message": "invalid ISO-8601 format" },
    { "field": "endAt", "message": "invalid ISO-8601 format" }
  ]
}
```

### Error Codes

| Status | 의미 | 발생 조건 |
|--------|------|-----------|
| 400 | Bad Request | JSON 파싱 실패, 유효성 검증 실패, 잘못된 UUID 형식 |
| 404 | Not Found | 존재하지 않는 일정 ID |
| 500 | Internal Server Error | 서버 내부 오류 |

### Validation Rules

| 필드 | 규칙 |
|------|------|
| title | 필수, 공백만으로는 불가 |
| startAt / endAt | 필수, ISO-8601 형식 (RFC 3339) |
| endAt | startAt 이후여야 함 |
| timezone | 유효한 IANA 타임존 식별자 |

---

## Event Object

| Field | Type | Nullable | Description |
|-------|------|----------|-------------|
| id | string (UUID) | No | 일정 고유 식별자 (자동 생성) |
| title | string | No | 일정 제목 |
| startAt | string (ISO-8601) | No | 시작 시간 (UTC) |
| endAt | string (ISO-8601) | No | 종료 시간 (UTC) |
| allDay | boolean | No | 종일 여부 |
| timezone | string | No | IANA 타임존 |
| locationName | string | Yes | 장소명 (없으면 응답에서 생략) |
| locationAddress | string | Yes | 주소 (없으면 응답에서 생략) |
| locationLat | number | Yes | 위도 (없으면 응답에서 생략) |
| locationLng | number | Yes | 경도 (없으면 응답에서 생략) |
| reminderOffsets | number[] | No | 알림 오프셋 배열 |
| notes | string | Yes | 메모 (없으면 응답에서 생략) |
| createdAt | string (ISO-8601) | No | 생성 시각 (UTC) |
| updatedAt | string (ISO-8601) | No | 수정 시각 (UTC) |
