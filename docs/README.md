# Tài liệu GoUp LMS

Bắt đầu từ [README gốc](../README.md) để chạy dev. Nguồn chân lý nghiệp vụ là [spec-lms-mvp.md](../spec-lms-mvp.md); kế hoạch triển khai ở [plans/](../plans/).

## Hệ thống

| Tài liệu | Nội dung |
|---|---|
| [architecture.md](architecture.md) | Layout monorepo, service compose, luồng request qua Caddy/nginx, nhóm biến môi trường, tầng nền backend, frontend |
| [database.md](database.md) | Schema, quyết định thiết kế, bất biến phiên bản ở tầng ứng dụng, hợp đồng outbox, migration, rollback, seed |
| `api.md` (thêm ở Phase 14) | Hợp đồng HTTP API |
| `runbook.md` (thêm ở Phase 14) | Vận hành production: triển khai, backup, rollback |

## Giao diện và prototype

| Tài liệu | Nội dung |
|---|---|
| [design.md](design.md) | Định hướng thiết kế và token GoUp |
| [prototype.md](prototype.md) | Cách chạy prototype tĩnh, tài khoản, route |
| [review.md](review.md) | Checklist nghiệm thu thay đổi prototype |
| [agents.md](agents.md) | Hướng dẫn agent làm việc với prototype |
| [prototype/](../prototype/) | Mã nguồn prototype (nguồn token `goup.css`) |
