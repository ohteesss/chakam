```markdown
# Chakam - WhatsApp Trust & Agreement Confirmation System

A WhatsApp-based system that creates timestamped, structured records of transaction agreements between two parties in Nigeria. Think of it as a "digital handshake" powered by AI.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## 🎯 What This Does

Chakam helps two parties confirm important transaction details on WhatsApp before, during, or after a transaction. It creates a clear, structured, timestamped record of what both parties agreed to.

### Core Use Cases

1. **Deal Confirmation** - Buyer and seller agree on item and price
2. **Payment Confirmation** - Record payment details and terms
3. **Delivery Confirmation** - Confirm delivery details and conditions
4. **Receipt Confirmation** - Acknowledge receipt of goods/services

### What It Is NOT

- ❌ Not an escrow system
- ❌ Not a legal enforcement tool
- ❌ Not a payment processor
- ❌ Not a legally binding contract

**It's a record-keeping service that creates proof of mutual agreement.**

## 🏗️ Architecture

### High-Level Overview

```
User (WhatsApp) 
    ↓
Twilio WhatsApp API
    ↓
Go Application (Gin)
    ↓
Gemini LLM (Conversation Orchestrator)
    ↓
Business Services → PostgreSQL
```

### Tech Stack

- **Language**: Go 1.21+
- **HTTP Framework**: Gin
- **ORM**: GORM
- **Database**: PostgreSQL 15+
- **AI/LLM**: Google Gemini (gemini-3.5)
- **Messaging**: Twilio WhatsApp API
- **Background Jobs**: Cron (MVP), Asynq (scale)
- **Logging**: Zap
- **Monitoring**: Prometheus + Sentry

### Architecture Style

**Modular Monolith with Clean Architecture**

```
├── Presentation Layer (HTTP Handlers)
├── Application Layer (LLM Orchestrator, Services)
├── Domain Layer (Business Logic, Models)
├── Infrastructure Layer (Database, External APIs)
```

## 🚀 Quick Start

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 15+
- Twilio account with WhatsApp enabled
- Google AI Studio account (Gemini API)
- Redis (optional, for caching)

### Installation

1. **Clone the repository**

```bash
git clone https://github.com/ohteesss/chakam.git
cd chakam
```

2. **Install dependencies**

```bash
go mod download
```

3. **Set up environment variables**

```bash
cp .env.example .env
# Edit .env with your credentials
```

4. **Set up the database**

```bash
# Create database
createdb chakam

# Run migrations
make migrate-up
```

5. **Run the application**

```bash
# Development mode
make dev

# Production mode
make run
```

### Environment Variables

```bash
# Server
PORT=8080
ENV=development

# Database
DATABASE_URL=postgres://user:pass@localhost:5432/chakam?sslmode=disable

# Twilio
TWILIO_ACCOUNT_SID=ACxxxxxxxxxxxxxxxxxxxxxxxxxxxx
TWILIO_AUTH_TOKEN=your_auth_token_here
TWILIO_PHONE_NUMBER=+14155238886

# Gemini AI
GEMINI_API_KEY=your_gemini_api_key_here

# Redis (optional)
REDIS_URL=redis://localhost:6379

# Security
JWT_SECRET=your_super_secret_jwt_key
ADMIN_API_KEY=your_admin_api_key

# Monitoring
SENTRY_DSN=https://your_sentry_dsn@sentry.io/project
```

## 📁 Project Structure

```
chakam/
│
├── cmd/
│   ├── api/              # Main application entry point
│   └── worker/           # Background job worker
│
├── internal/
│   ├── config/           # Configuration loading
│   │
│   ├── domain/           # Domain models and business logic
│   │   ├── user/
│   │   ├── session/
│   │   ├── request/
│   │   ├── response/
│   │   └── audit/
│   │
│   ├── infrastructure/   # External integrations
│   │   ├── database/
│   │   ├── repository/
│   │   ├── twilio/
│   │   └── gemini/
│   │
│   ├── interfaces/       # HTTP handlers and jobs
│   │   ├── http/
│   │   └── jobs/
│   │
│   ├── application/      # Application services
│   │   └── conversation/ # LLM orchestrator
│   │
│   └── pkg/              # Shared utilities
│
├── migrations/           # Database migrations
├── scripts/              # Helper scripts
├── tests/                # Tests
│
├── .env.example
├── Makefile
├── go.mod
└── README.md
```

## 🔄 How It Works

### Creating a Trust Request

```
1. User A sends: "I want to create a deal"
   
2. AI Assistant: "Great! What type of agreement?
   1) Sale
   2) Payment
   3) Delivery
   4) Receipt"
   
3. User A: "Sale"

4. AI Assistant: "What's the amount?"

5. User A: "50000"

6. AI Assistant: "Please describe what you're selling"

7. User A: "iPhone 13 Pro Max, 256GB, used"

8. AI Assistant: "Who is the buyer? (phone number)"

9. User A: "08012345678"

10. System creates request and sends to User B:
    "John wants to confirm a deal with you:
     Type: Sale
     Amount: ₦50,000
     Item: iPhone 13 Pro Max, 256GB, used
     
     Reply CONFIRM to accept or REJECT to decline"

11. User B: "CONFIRM"

12. Both parties receive confirmation with timestamp
```

### Request Lifecycle

```
PENDING → [Counterparty confirms] → CONFIRMED ✅
        → [Counterparty rejects]  → REJECTED ❌
        → [48hr timeout]          → EXPIRED ⏱️
        → [Initiator cancels]     → CANCELLED 🚫
        → [Dispute raised]        → DISPUTED ⚠️
```

## 🗄️ Database Schema

### Core Tables

**users** - User profiles and verification status
```sql
- id, public_id, phone_number
- name, is_verified, verification_data
- is_blocked, blocked_reason
```

**sessions** - Conversation state and history
```sql
- id, user_id
- conversation_history (JSONB)
- last_message_at, expires_at
```

**trust_requests** - Agreement records
```sql
- id, public_id
- initiator_id, counterparty_id
- request_type, amount_kobo, description
- status, expires_at
- confirmed_at, rejected_at, cancelled_at
```

**request_responses** - Confirmation/rejection records
```sql
- id, request_id, respondent_id
- action (CONFIRMED/REJECTED)
- notes, created_at
```

**audit_logs** - Complete audit trail
```sql
- id, event_type, event_data
- user_id, request_id
- ip_address, created_at
```

## 🤖 LLM Integration (Gemini)

### Why Gemini?

- ✅ Natural language understanding
- ✅ Multi-language support (English, Pidgin, Yoruba, Igbo, Hausa)
- ✅ Function calling for structured actions
- ✅ Cost-effective (~$20-50/month for MVP scale)
- ✅ Fast inference (1-2s response time)

### Available Functions

The AI can call these functions:

1. **create_trust_request** - Create new agreement
2. **confirm_request** - Accept pending request
3. **reject_request** - Decline pending request
4. **get_request_status** - Check request status
5. **cancel_request** - Cancel own request

### Fallback Strategy

If Gemini API fails or times out, the system falls back to rule-based responses for core commands:
- CONFIRM
- REJECT
- HELP
- START

## 📡 API Endpoints

### Webhooks

```
POST   /webhook/twilio/incoming   # Receive messages
POST   /webhook/twilio/status     # Message delivery status
```

### Public API

```
GET    /api/v1/requests           # List user's requests
GET    /api/v1/requests/:id       # Get request details
PATCH  /api/v1/requests/:id/cancel # Cancel request
```

### Admin API

```
POST   /api/admin/users/:id/block # Block user
GET    /api/admin/audit-logs      # Query audit logs
GET    /api/admin/stats           # Usage statistics
```

### Health & Metrics

```
GET    /health                    # Health check
GET    /metrics                   # Prometheus metrics
```

## 🔒 Security Features

### Request Security
- ✅ Twilio signature verification
- ✅ Rate limiting (10 req/min per IP)
- ✅ Message deduplication (replay protection)
- ✅ Admin endpoint authentication (JWT)

### Data Protection
- ✅ Phone number masking in logs
- ✅ Sensitive data encryption at rest
- ✅ No storage of full message content
- ✅ GDPR-compliant audit logging

### Abuse Prevention
- ✅ Daily request limits per user (20/day)
- ✅ User blocking capability
- ✅ Spam detection
- ✅ Request expiry (48 hours)

## 🔍 Monitoring & Observability

### Logging

Uses structured logging with Zap:

```go
logger.Info("request created",
    zap.String("request_id", req.PublicID),
    zap.String("type", string(req.RequestType)),
)
```

### Metrics

Key business metrics tracked:
- Total requests created
- Requests by status (pending, confirmed, rejected)
- Confirmation rate
- Average time to confirmation
- Daily active users
- LLM call latency and errors

### Error Monitoring

Integrated with Sentry for error tracking and alerting.

## 🧪 Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run integration tests
make test-integration

# Run linter
make lint
```

## 📦 Deployment

### Using Docker

```bash
# Build image
docker build -t trustconfirm:latest .

# Run container
docker run -p 8080:8080 \
  --env-file .env \
  trustconfirm:latest
```

### Using Docker Compose

```bash
docker-compose up -d
```

### Manual Deployment

```bash
# Build binary
make build

# Run migrations
./scripts/migrate.sh up

# Start application
./bin/trustconfirm-api

# Start worker (separate process)
./bin/trustconfirm-worker
```

## 📈 Scaling Considerations

### Current Capacity (Single Server)

- ✅ 0-10,000 requests/day
- ✅ ~1,000 concurrent users
- ✅ Sub-3s response time (p95)

### Scaling Path

**Phase 1: Vertical Scaling**
- Bigger server (4 → 8 CPU)
- Database connection pooling
- Redis for session caching

**Phase 2: Horizontal Scaling**
- Load balancer (Nginx/AWS ALB)
- Multiple app instances
- Database read replicas

**Phase 3: Service Extraction**
- Separate notification service
- Separate LLM service
- Event-driven architecture (Kafka/NATS)


## 🤝 Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Code Standards

- Follow Go best practices
- Write tests for new features
- Update documentation
- Run `make lint` before committing

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## ⚠️ Disclaimer

**IMPORTANT**: This service creates records of agreements, NOT legally binding contracts. It does not:
- Provide legal advice
- Enforce agreements
- Hold funds (escrow)
- Guarantee payment or delivery
- Replace legal contracts or courts

Users should seek proper legal advice for formal agreements.



---

**Built with ❤️ in Nigeria 🇳🇬**

**Making trust tangible, one agreement at a time.**
