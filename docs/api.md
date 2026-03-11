# Haru API Documentation

Base URL: `http://localhost:8080`

---

## 인증 (Authentication)

Event, Voice Parsing 등 대부분의 API는 인증이 필요합니다. 로그인 후 발급받은 access token을 `Authorization` 헤더에 포함하세요.

```
Authorization: Bearer {accessToken}
```

| 분류 | 엔드포인트 | 인증 필요 |
|------|-----------|----------|
| Public | `POST /api/auth/apple`, `POST /api/auth/refresh` | X |
| Protected | 그 외 모든 `/api/*` 엔드포인트 | O |
| Health | `GET /health` | X |

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

## Auth API

### 1. Apple 로그인

Apple Sign In SDK에서 받은 `id_token`을 서버에 전달하면, 서버가 Apple 공개키로 검증 후 사용자를 조회/생성하고 JWT 토큰 쌍을 발급합니다.

```
POST /api/auth/apple
```

**Request Body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| idToken | string | O | Apple Sign In에서 받은 id_token (JWT) |
| code | string | | Apple authorization code (현재 미사용) |

**Example**

```bash
curl -X POST http://localhost:8080/api/auth/apple \
  -H "Content-Type: application/json" \
  -d '{
    "idToken": "eyJraWQiOiJmaDZCcz..."
  }'
```

**Response** `200 OK`

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIs...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIs...",
  "expiresIn": 3600,
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "provider": "apple",
    "email": "user@privaterelay.appleid.com",
    "createdAt": "2024-03-06T12:00:00Z",
    "lastLoginAt": "2024-03-06T12:00:00Z"
  }
}
```

**Error**

| Status | 조건 |
|--------|------|
| 400 | `idToken` 누락 |
| 401 | id_token 검증 실패 (서명 불일치, 만료 등) |
| 502 | Apple 공개키 조회 실패 |

> **Note:** Apple은 최초 로그인 시에만 email을 제공합니다. 이후 로그인에서는 `sub`(사용자 ID)만 반환되므로, 최초 로그인 시 서버가 자동으로 email을 저장합니다.

---

### 2. 토큰 갱신

Refresh Token Rotation 방식으로 새 토큰 쌍을 발급합니다. 기존 refresh token은 즉시 폐기됩니다.

```
POST /api/auth/refresh
```

**Request Body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| refreshToken | string | O | 이전에 발급받은 refresh token |

**Example**

```bash
curl -X POST http://localhost:8080/api/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refreshToken": "eyJhbGciOiJIUzI1NiIs..."
  }'
```

**Response** `200 OK`

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIs...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIs...",
  "expiresIn": 3600
}
```

**Error**

| Status | 조건 |
|--------|------|
| 400 | `refreshToken` 누락 |
| 401 | 유효하지 않거나 만료된 refresh token |

---

### 3. 현재 사용자 정보 조회

```
GET /api/auth/me
```

**Headers:** `Authorization: Bearer {accessToken}`

**Example**

```bash
curl http://localhost:8080/api/auth/me \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

**Response** `200 OK`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "provider": "apple",
  "email": "user@privaterelay.appleid.com",
  "createdAt": "2024-03-06T12:00:00Z",
  "lastLoginAt": "2024-03-10T09:30:00Z"
}
```

**Error**

| Status | 조건 |
|--------|------|
| 401 | 토큰 누락 또는 만료 |
| 404 | 사용자를 찾을 수 없음 |

---

### 4. 로그아웃

해당 사용자의 모든 refresh token을 삭제합니다.

```
POST /api/auth/logout
```

**Headers:** `Authorization: Bearer {accessToken}`

**Example**

```bash
curl -X POST http://localhost:8080/api/auth/logout \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

**Response** `204 No Content`

(응답 본문 없음)

---

### 5. 회원 탈퇴

사용자의 모든 refresh token과 사용자 계정을 삭제합니다.

```
DELETE /api/auth/account
```

**Headers:** `Authorization: Bearer {accessToken}`

**Example**

```bash
curl -X DELETE http://localhost:8080/api/auth/account \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

**Response** `204 No Content`

(응답 본문 없음)

---

## Event API

> 모든 Event API는 인증이 필요합니다. `Authorization: Bearer {accessToken}` 헤더를 포함하세요.
> 각 사용자는 자신의 일정만 조회/수정/삭제할 수 있습니다.

### 6. 일정 생성

```
POST /api/events
```

**Headers:** `Authorization: Bearer {accessToken}`

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
| reminderOffsets | number[] | | 알림 오프셋 배열 (분 단위, default: `[180]` = 3시간 전). 빈 배열 `[]`을 명시하면 알림 없음 |
| notes | string \| null | | 메모 |

**Example**

```bash
curl -X POST http://localhost:8080/api/events \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
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

### 7. 일정 단건 조회

```
GET /api/events/:id
```

**Headers:** `Authorization: Bearer {accessToken}`

**Path Parameters**

| Parameter | Type | Description |
|-----------|------|-------------|
| id | UUID | 일정 ID |

**Example**

```bash
curl http://localhost:8080/api/events/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
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

### 8. 일정 목록 조회 (날짜 범위)

```
GET /api/events?start={start}&end={end}
```

**Headers:** `Authorization: Bearer {accessToken}`

**Query Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| start | string | O | 조회 시작 시간 (ISO-8601) |
| end | string | O | 조회 종료 시간 (ISO-8601) |

**Example**

```bash
curl "http://localhost:8080/api/events?start=2024-03-01T00:00:00Z&end=2024-03-31T23:59:59Z" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
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

### 9. 일정 수정

```
PUT /api/events/:id
```

**Headers:** `Authorization: Bearer {accessToken}`

**Path Parameters**

| Parameter | Type | Description |
|-----------|------|-------------|
| id | UUID | 일정 ID |

**Request Body**

일정 생성과 동일한 필드 구조.

**Example**

```bash
curl -X PUT http://localhost:8080/api/events/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
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

### 10. 일정 삭제

```
DELETE /api/events/:id
```

**Headers:** `Authorization: Bearer {accessToken}`

**Path Parameters**

| Parameter | Type | Description |
|-----------|------|-------------|
| id | UUID | 일정 ID |

**Example**

```bash
curl -X DELETE http://localhost:8080/api/events/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

**Response** `204 No Content`

(응답 본문 없음)

---

## Voice Parsing API

> 인증이 필요합니다. `Authorization: Bearer {accessToken}` 헤더를 포함하세요.

### 11. 음성 텍스트 → 일정 파싱

한국어 음성 텍스트를 AI(Gemini)로 분석하여 구조화된 일정 데이터를 추출합니다.
응답의 `event` 필드는 `POST /api/events`의 요청 본문과 동일한 구조이므로, 그대로 일정 생성에 사용할 수 있습니다.

```
POST /api/events/parse-voice
```

**Headers:** `Authorization: Bearer {accessToken}`

**Request Body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| text | string | O | 음성 인식된 한국어 텍스트 |

**Example**

```bash
curl -X POST http://localhost:8080/api/events/parse-voice \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{
    "text": "다음주 수요일 오후 3시에 서초동 교대역 근처 스타벅스에서 김대리랑 프로젝트 회의하는데 30분 전에 알려줘"
  }'
```

**Response** `200 OK`

```json
{
  "event": {
    "title": "프로젝트 회의",
    "startAt": "2024-03-13T15:00:00+09:00",
    "endAt": "2024-03-13T16:00:00+09:00",
    "allDay": false,
    "timezone": "Asia/Seoul",
    "locationName": "스타벅스",
    "locationAddress": null,
    "locationLat": null,
    "locationLng": null,
    "reminderOffsets": [30],
    "notes": "김대리랑. 서초동 교대역 근처."
  },
  "confidence": 0.9,
  "followUpQuestion": null
}
```

**Response Fields**

| Field | Type | Description |
|-------|------|-------------|
| event | CreateEventRequest | 파싱된 일정 데이터 (`POST /api/events` 요청 본문과 동일 구조) |
| confidence | number | 파싱 신뢰도 (0.0~1.0). 0.5 미만이면 `followUpQuestion` 확인 권장 |
| followUpQuestion | string \| null | 정보 부족 시 AI가 생성하는 후속 질문 (한국어) |

**AI 기본값 규칙**

| 누락 정보 | 기본값 |
|-----------|--------|
| 날짜 | 오늘 |
| 종료 시간 | 시작 시간 + 1시간 |
| 알림 | `[10]` (10분 전) |
| 장소 | null |
| 메모 | null |

> **사용 흐름:** parse-voice → 사용자 확인/수정 → `POST /api/events`로 저장

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
| 401 | Unauthorized | 인증 토큰 누락, 만료, 검증 실패 |
| 404 | Not Found | 존재하지 않는 일정 ID 또는 사용자 |
| 422 | Unprocessable Entity | 음성 텍스트에서 일정 정보 추출 실패 |
| 500 | Internal Server Error | 서버 내부 오류 |
| 502 | Bad Gateway | AI 서비스(Gemini) 또는 Apple 인증 서버 호출 실패 |

### Validation Rules

| 필드 | 규칙 |
|------|------|
| title | 필수, 공백만으로는 불가 |
| startAt / endAt | 필수, ISO-8601 형식 (RFC 3339) |
| endAt | startAt 이후여야 함 |
| timezone | 유효한 IANA 타임존 식별자 |
| idToken | Apple 로그인 시 필수 |
| refreshToken | 토큰 갱신 시 필수 |

---

## Data Objects

### User Object

| Field | Type | Nullable | Description |
|-------|------|----------|-------------|
| id | string (UUID) | No | 사용자 고유 식별자 |
| provider | string | No | OAuth 제공자 (`"apple"`) |
| email | string | Yes | 이메일 (없으면 응답에서 생략) |
| nickname | string | Yes | 닉네임 (없으면 응답에서 생략) |
| profileImage | string | Yes | 프로필 이미지 URL (없으면 응답에서 생략) |
| createdAt | string (ISO-8601) | No | 가입 시각 |
| lastLoginAt | string (ISO-8601) | Yes | 마지막 로그인 시각 (없으면 응답에서 생략) |

### Auth Response Object

| Field | Type | Nullable | Description |
|-------|------|----------|-------------|
| accessToken | string | No | JWT access token (유효기간: 1시간) |
| refreshToken | string | No | JWT refresh token (유효기간: 30일) |
| expiresIn | number | No | access token 만료까지 남은 초 |
| user | User Object | Yes | 사용자 정보 (로그인 시에만 포함, 갱신 시 생략) |

### Event Object

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
| reminderOffsets | number[] | No | 알림 오프셋 배열 (분 단위, 미지정 시 `[180]`) |
| notes | string | Yes | 메모 (없으면 응답에서 생략) |
| createdAt | string (ISO-8601) | No | 생성 시각 (UTC) |
| updatedAt | string (ISO-8601) | No | 수정 시각 (UTC) |

---

## 환경변수

| 변수명 | 필수 | 기본값 | 설명 |
|--------|------|--------|------|
| `PORT` | X | `8080` | 서버 포트 |
| `DATABASE_URL` | X | - | PostgreSQL 연결 URL |
| `DB_DRIVER` | X | `sqlite` | DB 드라이버 (`sqlite` 또는 `postgres`) |
| `JWT_SECRET` | **O** | - | JWT 서명 비밀키 (미설정 시 서버 시작 차단) |
| `JWT_ACCESS_EXPIRY` | X | `1h` | Access token 유효기간 |
| `JWT_REFRESH_EXPIRY` | X | `720h` | Refresh token 유효기간 (30일) |
| `APPLE_CLIENT_ID` | X | - | Apple App Bundle ID (e.g., `com.deepfine.haru`) |
| `APPLE_TEAM_ID` | X | - | Apple Developer Team ID |
| `APPLE_KEY_ID` | X | - | Apple Sign In Key ID |
| `APPLE_PRIVATE_KEY` | X | - | Apple private key (PEM 문자열) |
| `GEMINI_API_KEY` | X | - | Gemini API 키 (미설정 시 음성 파싱 502 반환) |
| `GEMINI_MODEL` | X | `gemini-2.5-flash` | Gemini 모델명 |
| `DEFAULT_TIMEZONE` | X | `Asia/Seoul` | 음성 파싱 기본 타임존 |

---

## JWT 토큰 설계

### Access Token

| 항목 | 값 |
|------|-----|
| 알고리즘 | HS256 |
| 유효기간 | 1시간 (기본) |
| Payload | `jti` (고유 ID), `sub` (user ID), `iat`, `exp` |

### Refresh Token

| 항목 | 값 |
|------|-----|
| 알고리즘 | HS256 |
| 유효기간 | 30일 (기본) |
| 저장 | DB (`refresh_tokens` 테이블) |
| 정책 | Refresh Token Rotation (갱신 시 기존 토큰 폐기) |
