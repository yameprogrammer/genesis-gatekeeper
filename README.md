# 🛡️ Genesis Gatekeeper

**Next-Gen Multi-platform Solution Architect for Private Server Management.**

`Genesis Gatekeeper`는 한정된 리소스(Android/Termux, On-premise) 환경에서 서버의 보안을 극대화하고 시스템 자원을 지능적으로 감시하기 위해 설계된 고성능 보안 관리 봇입니다.

## ✨ Key Features

### 1. Stealth Security (Silent Gatekeeper)
- **Silent Drop:** 비인가 사용자의 모든 접근을 무응답으로 일관하여 정찰 활동을 원천 차단합니다.
- **Anti-Reconnaissance Jitter:** 응답 지연 시간에 랜덤 지터(1~3초)를 부여하여 봇 여부 판별을 어렵게 합니다.
- **Auth Logging:** 모든 비인가 접근 시도를 SQLite DB에 기록하여 사후 분석을 지원합니다.

### 2. Intelligent Health Monitoring
- **Resource Tracking:** CPU, Memory, Disk, Battery 상태를 5분 주기로 정밀 모니터링합니다.
- **Smart Alerting:** 30분 쿨다운 로직을 통해 동일 장애에 대한 알림 피로도를 최소화합니다.
- **Robust Fallback:** OS 레벨의 쉘 명령 실패 시 Go Native System Call(`syscall`)을 통해 데이터 무결성을 보장합니다.

### 3. Automated Reporting
- **Daily Security Briefing:** 매일 아침 9시(KST), 전날의 보안 위협과 현재 시스템 상태를 HTML 리포트로 요약 발송합니다.

## 🚀 Technical Stack
- **Language:** Go (Golang)
- **Database:** SQLite (Pure Go, CGO-free for portability)
- **Environment:** Optimized for Termux (Android/ARM64) & Linux On-premise
- **Libraries:** `telebot.v3`, `robfig/cron/v3`, `modernc.org/sqlite`

## 🛠️ Installation & Build

```bash
# Build for Android/ARM64 (Termux)
CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -o genesis-gatekeeper ./cmd/genesis-gatekeeper