# Database Guide

## 현재 마이그레이션

GORM `AutoMigrate`로 자동 생성됩니다. (`cmd/server/main.go`)

```go
db.AutoMigrate(
    &model.User{}, &model.RefreshToken{}, &model.Event{},
    &model.Notification{}, &model.DeviceToken{},
)
```

---

## 수동 인덱스 (프로덕션 배포 시 추가)

GORM AutoMigrate가 생성하는 단일 인덱스 외에, 프로덕션 환경에서 쿼리 성능을 위해 아래 인덱스를 수동으로 추가하세요.

### notifications 테이블

`NotificationWorker`가 1분 간격으로 실행하는 `FindPending` 쿼리:

```sql
SELECT * FROM notifications
WHERE sent = false AND notify_at <= now() AND retries < 3
ORDER BY notify_at ASC
LIMIT 100;
```

복합 인덱스:

```sql
CREATE INDEX idx_notifications_pending
    ON notifications (sent, notify_at, retries)
    WHERE sent = false;
```

> `WHERE sent = false` partial index를 사용하면 발송 완료된 행을 인덱스에서 제외하여 크기를 줄일 수 있습니다. SQLite는 partial index를 지원하지만, 로컬 개발에서는 데이터량이 적으므로 없어도 무방합니다.

### events 테이블

날짜 범위 조회 (`GET /api/events?start=...&end=...`):

```sql
SELECT * FROM events
WHERE user_id = ? AND start_at < ? AND end_at > ?
ORDER BY all_day DESC, start_at ASC;
```

복합 인덱스:

```sql
CREATE INDEX idx_events_user_date_range
    ON events (user_id, start_at, end_at);
```

> GORM AutoMigrate가 `start_at`, `end_at` 각각에 단일 인덱스를 생성하지만, `user_id`를 포함한 복합 인덱스가 멀티테넌시 쿼리에 더 효율적입니다.
