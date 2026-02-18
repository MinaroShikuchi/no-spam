# no-spam: Secure Notification Backend

A production-ready, modular notification system in Go using the **Hub & Connector** pattern.
Designed for security (TLS 1.3, JWT) and extensibility.

## Features

- **Hub & Connector Pattern**: Centralized routing with hot-swappable providers.
- **Connectors**:
  - **Mock**: For testing.
  - **FCM**: Firebase Cloud Messaging.
  - **APNS**: Apple Push Notification Service.
  - **Webhook**: Generic HTTP POST integration (e.g., Discord/Slack/Custom).
- **Security**:
  - **JWT Middleware**: Enforces signed tokens on API endpoints.
  - **RBAC**: Role-based access control (`admin`, `publisher`, `subscriber`).
  - **TLS 1.3 Strict**: Configured to reject older protocols.
- **Admin**:
  - Auto-generated admin user on startup.
  - Token generation via API.
  - Topic inspection and management.

## Getting Started

### Prerequisites

- Go 1.22+
- TLS Certificates (`cert.pem`, `key.pem`) for local development.

### Installation

```bash
git clone <repo>
cd no-spam
go mod download
```

### Running the Server

Since the server enforces TLS 1.3, you need certificates. For local testing:

```bash
# Generate self-signed certs (requires openssl)
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
```

Run the server:

```bash
export JWT_SECRET="your-secret-key"
go run .
```

Or build and run:

```bash
go build -o no-spam .
./no-spam
```

**First Run**:
- If no admin user exists, the server creates one and logs credentials.
- If certificates are missing, the server **auto-generates** self-signed certs in `certs/` directory (unless `-http` is used).

#### Flags
- `-addr`: Address to listen on (default `:8443`)
- `-cert`: Path to cert file (default `certs/cert.pem`)
- `-key`: Path to key file (default `certs/key.pem`)
- `-fcm-creds`: Path to Firebase Service Account JSON (optional)
- `-http`: Run in HTTP mode (disable TLS). Useful for reverse proxies.

### Authentication

All API endpoints (except login/register) require a **Bearer Token**.
Roles:
- **subscriber**: Can subscribe/unsubscribe.
- **publisher**: Can publish messages.
- **admin**: Full access to admin endpoints.

#### 1. Public Endpoints
- **POST** `/login`: Get JWT token.
- **POST** `/register`: Create new user (default role: `subscriber`).

#### 2. Admin Token Generation
Admins can generate tokens for any role:
**GET** `/admin/token?role=publisher`
Headers: `Authorization: Bearer <admin-token>`

### API Usage

#### Send Notification (Publisher)
**POST** `/send`
Headers: `Authorization: Bearer <publisher-token>`

```json
{
  "provider": "mock",
  "token": "user-device-token",
  "payload": "SGVsbG8gV29ybGQ=" // Base64 encoded payload
}
``` 

#### Subscribe to Topic (Subscriber)
**POST** `/subscribe`
Headers: `Authorization: Bearer <subscriber-token>`

```json
{
  "topic": "alerts",
  "token": "user-device-token",
  "provider": "fcm"
}
```

#### Subscribe with Webhook
**POST** `/subscribe`
Headers: `Authorization: Bearer <subscriber-token>`

```json
{
  "topic": "alerts",
  "provider": "webhook",
  "webhook": "https://discord.com/api/webhooks/..."
}
```

> **Note**: The published payload for a webhook provider must match the format expected by the webhook service (e.g., for Discord, it must be `{"content": "message"}`).

**History Replay**: Upon subscribing, the last 20 messages for the topic are immediately queued for delivery.

### Admin API

Requires `role: admin`.

- **GET** `/admin/topics`: List all topics.
- **POST** `/admin/topics`: Create a topic.
- **DELETE** `/admin/topics/:name`: Delete a topic (must be empty).
- **GET** `/admin/topics/:name/messages`: Inspect topic queue.
- **GET** `/admin/topics/:name/subscribers`: List subscribers.

## Extensibility

To add a new connector:

1. **Create the implementation** in `connectors/myconnector.go` satisfying the `Connector` interface.
2. **Register it** in `main.go`:
   ```go
   myConn := connectors.NewMyConnector()
   hub.RegisterConnector("my-provider", myConn)
   ```
3. **Use it**: Send messages with `"provider": "my-provider"`.
