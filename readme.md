# Chakam - WhatsApp Trust & Agreement Confirmation System

A WhatsApp-based system that creates timestamped, structured records of transaction agreements between two parties in Nigeria. Think of it as a "digital handshake" powered by AI.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
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
User ↔ Meta WhatsApp Cloud API
              ↓ webhook
        API service (Gin)
              ↓ durable task
            Redis
              ↓
       Worker service (Asynq)
         ↙       ↓       ↘
     Gemini  PostgreSQL  Meta Graph API
```

### Tech Stack

- **Language**: Go 1.25+
- **HTTP Framework**: Gin
- **ORM**: GORM
- **Database**: PostgreSQL 15+
- **AI/LLM**: Google Gemini
- **Messaging**: Meta WhatsApp Cloud API
- **Background Jobs**: Asynq plus an in-process scheduled-job runner
- **Logging**: Zap
- **Operations**: Structured logs, liveness, and dependency readiness checks

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

- Go 1.25 or higher
- PostgreSQL 15+
- Meta developer account with a WhatsApp Business Account and registered phone number
- Google AI Studio account (Gemini API)
- Redis 7+ (required for the durable task queue)

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

# API and worker run as separate processes
make run
make run-worker
```

### Environment Variables

Copy [.env.example](.env.example) for the complete list. In production, the
application validates all Meta, Gemini, internal API, database, and queue
settings before starting. `DATABASE_URL` must use TLS and `INTERNAL_API_KEY`
must contain at least 32 characters.

For least privilege, give the API only the webhook verify token, Meta app
secret, and internal API key. Give the worker only the Meta access token, phone
number ID, and Gemini key. Both processes need PostgreSQL and Redis settings.

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
│   │   ├── agreement/
│   │   ├── response/
│   │   └── audit/
│   │
│   ├── infrastructure/   # External integrations
│   │   ├── database/
│   │   │   └── migrations/
│   │   ├── repository/
│   │   ├── whatsapp/
│   │   ├── gemini/
│   │   └── queue/
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
├── .env.example
├── compose.yml
├── Dockerfile
├── makefile
├── go.mod
└── README.md
```

## 🔄 How It Works

### Creating an Agreement

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

10. System creates the agreement and sends to User B:
    "John wants to confirm a deal with you:
     Type: Sale
     Amount: ₦50,000
     Item: iPhone 13 Pro Max, 256GB, used
     
     Reply CONFIRM to accept or REJECT to decline"

11. User B: "CONFIRM"

12. The responder receives a timestamped confirmation with the agreement reference
```

### Agreement Lifecycle

```
PENDING → [Counterparty confirms] → CONFIRMED ✅
        → [Counterparty rejects]  → REJECTED ❌
        → [Initiator cancels]     → CANCELLED 🚫
```

## 🗄️ Database Schema

### Core Tables

**users** - User profiles and verification status
```sql
- id, public_id, phone_number
- name, metadata
- is_blocked, blocked_reason
- created_at, updated_at
```

**sessions** - Conversation state and history
```sql
- id, user_id
- conversation_history (JSONB)
- expires_at, created_at, updated_at
```

**agreements** - Agreement records
```sql
- id
- initiator_id, counterparty_id
- agreement_type, amount_minor, currency, description
- status, created_at, updated_at
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

1. **create_agreement** - Create new agreement
2. **confirm_agreement** - Accept pending agreement
3. **get_my_agreements** - List a user's agreements

### Fallback Strategy

Queue jobs use bounded retries when Gemini or an outbound dependency fails.
A rule-based conversational fallback is not implemented yet.

## 📡 API Endpoints

### Webhooks

```
GET    /webhook/whatsapp          # Meta webhook verification handshake
POST   /webhook/whatsapp          # Receive messages and delivery-status events
```

### Internal API

These routes require `X-API-Key: <INTERNAL_API_KEY>`.

```
GET    /api/v1/agreements         # List user's agreements
GET    /api/v1/agreements/:id     # Get agreement details
GET    /api/v1/users              # List users (page and page_size query parameters)
POST   /send-message              # Queue an outbound text message
```

`GET /api/v1/users` defaults to `page=1` and `page_size=20`; `page_size` may
be between 1 and 100. The response includes `users` plus `pagination` metadata
(`page`, `page_size`, `total`, `total_pages`, and `has_next`).

### Health

```
GET    /health                    # Backward-compatible liveness check
GET    /livez                     # Process liveness
GET    /readyz                    # PostgreSQL and Redis readiness
```

## 🔒 Security Features

### Request Security
- ✅ Meta webhook signature verification (`X-Hub-Signature-256`)
- ✅ Redis task IDs deduplicate repeated Meta message IDs
- ✅ Internal endpoints protected by a constant-time API-key check
- ✅ Request-body and HTTP-header size limits
- ✅ Production startup rejects missing or unsafe secrets

### Data Protection
- ✅ Phone number masking in logs
- ✅ Production PostgreSQL configuration requires transport encryption
- ✅ Containers run as a non-root user with a read-only application filesystem
- ⚠️ Conversation content is retained until session expiry; configure database encryption at rest with the hosting provider

Rate limiting, immutable audit records, daily quotas, and spam controls remain
application-level follow-up work.

## 🔍 Monitoring & Observability

### Logging

Uses structured logging with Zap:

```go
logger.Info("agreement created",
    zap.String("agreement_id", agr.ID),
    zap.String("type", agr.AgreementType),
)
```

Production application logs are JSON through Zap. They include process version,
request/task identifiers, dependency failures, and job duration without full
phone numbers or message bodies; SQL logging is disabled in production. Asynq
emits its own worker lifecycle lines. Metrics and hosted error monitoring are
not yet integrated.

## 🧪 Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run with the race detector
make test-race

# Run formatting, vet, tests, and command builds
make verify
```

## 📦 Deployment

### Build the production image

```bash
# Build image
docker build -t chakam:latest .
docker build --target migrations -t chakam-migrations:latest .

# Run container
docker run --rm -p 8080:8080 --env-file .env chakam:latest /app/api

# Start the worker as a second deployment using the same immutable image
docker run --rm --env-file .env chakam:latest /app/worker
```

### Local stack with Docker Compose

```bash
docker compose up --build -d
docker compose ps
curl --fail http://localhost:8080/readyz
```

### Manual Deployment

```bash
# Build binary
make build

# Run versioned migrations before starting either process
make migrate-up

# Start application
./bin/chakam-api

# Start worker (separate process)
./bin/chakam-worker
```

### Production rollout

1. Build and scan one immutable image, then deploy that same digest for API and worker.
2. Store all credentials in the platform's secret manager; do not deploy `.env`.
3. Set `ENV=production`, use TLS for PostgreSQL, and enable `REDIS_TLS` when required by the provider.
4. Run `migrate-up` as a one-off release job before rolling out either process.
5. Route traffic only after `/readyz` returns 200. Use `/livez` for liveness.
6. Give API at least 20 seconds and worker at least 30 seconds to handle `SIGTERM`.
7. Back up PostgreSQL before a migration. Roll back the application image first; run a down migration only after confirming it is data-safe.

## 📈 Scaling Considerations

### Scaling Path

**Phase 1: Vertical Scaling**
- Bigger server (4 → 8 CPU)
- Database connection pooling
- Redis queue capacity and worker concurrency tuning

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
