# no-spam: Secure Notification Backend

A production-ready, modular notification system in Go using the **Hub & Connector** pattern.
Designed for security (TLS 1.3, JWT) and extensibility.

## Features

- **Hub & Connector Pattern**: Centralized routing with hot-swappable providers.
- **Connectors**:
  - **Mock**: For testing.
  - **FCM**: Skeleton for Firebase Cloud Messaging.
  - **APNS**: Skeleton for Apple Push Notification Service.
  - **WebSocket**: Real-time connections using `nhooyr.io/websocket`.
- **Security**:
  - **JWT Middleware**: Enforces signed tokens on API and WebSockets.
  - **TLS 1.3 Strict**: Configured to reject older protocols.

## Getting Started

### Prerequisites

- Go 1.22+
- TLS Certificates (`cert.pem`, `key.pem`) for local development (or use flags).

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
go run main.go hub.go
```

Or build and run:

```bash
go build -o no-spam .
./no-spam
```

Flags:
- `-addr`: Address to listen on (default `:8443`)
- `-cert`: Path to cert file (default `cert.pem`)
- `-key`: Path to key file (default `key.pem`)

### API Usage

#### Send Notification
**POST** `/send`
Headers: `Authorization: Bearer <valid-jwt>`

```json
{
  "provider": "mock",
  "token": "user-device-token",
  "payload": "SGVsbG8gV29ybGQ=" // Base64 encoded payload, or raw bytes if JSON allows
}
``` 
*Note: Payload type in Go struct is `[]byte`, relying on JSON base64 encoding/decoding automatically.*

#### WebSocket Connection
**GET** `/ws?token=<valid-jwt>`
- Connect with a WebSocket client.
- Messages sent to `provider: websocket` with this token will be pushed to this socket.

#### Subscribe to Topic
**POST** `/subscribe`
Headers: `Authorization: Bearer <valid-jwt>`

```json
{
  "topic": "alerts",
  "token": "user-device-token",
  "provider": "fcm"
}
```

#### Broadcast Message
**POST** `/send`
Headers: `Authorization: Bearer <valid-jwt>`

```json
{
  "topic": "alerts",
  "payload": "Emergency Broadcast"
}
```



#### Unsubscribe from Topic
**POST** `/unsubscribe`
Headers: `Authorization: Bearer <valid-jwt>`

```json
{
  "topic": "alerts",
  "token": "user-device-token"
}
```

### Role-Based Access Control (RBAC)

Tokens must have a `role` claim (`publisher` or `subscriber`).

**Generate Publisher Token**:
```bash
go run cmd/token-gen/main.go -role publisher
```

**Generate Subscriber Token**:
```bash
go run cmd/token-gen/main.go -role subscriber
```

- **Publishers**: Authorized to `/send`.
- **Subscribers**: Authorized to `/subscribe` and connect to WebSocket.

1. **Create the implementation** in `connectors/myconnector.go`:
   ```go
   package connectors
   import "context"

   type MyConnector struct {}
   
   func NewMyConnector() *MyConnector { return &MyConnector{} }

   func (m *MyConnector) Send(ctx context.Context, token string, payload []byte) error {
       // Implement logic
       return nil
   }
   ```

2. **Register it** in `main.go`:
   ```go
   myConn := connectors.NewMyConnector()
   hub.RegisterConnector("my-provider", myConn)
   ```

3. **Use it**: Send messages with `"provider": "my-provider"`.
