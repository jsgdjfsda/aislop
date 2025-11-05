# User Management Service

Authentication and authorization service with JWT tokens and multi-tenant home management.

## Features

- User registration and authentication
- JWT token generation and validation
- Password hashing with bcrypt
- Token refresh mechanism
- User profile management
- Multi-tenant home/location support
- Prometheus metrics

## API Endpoints

### Register
```bash
POST /api/auth/register
{
  "email": "user@example.com",
  "password": "securepass123",
  "full_name": "John Doe",
  "phone": "+1234567890"
}
```

### Login
```bash
POST /api/auth/login
{
  "email": "user@example.com",
  "password": "securepass123"
}
```

Returns JWT token for authentication.

### Get Profile
```bash
GET /api/users/profile
Authorization: Bearer <token>
```

### Update Profile
```bash
PUT /api/users/profile
Authorization: Bearer <token>
{
  "full_name": "Jane Doe",
  "phone": "+1987654321"
}
```

### List Homes
```bash
GET /api/users/homes
Authorization: Bearer <token>
```

### Create Home
```bash
POST /api/users/homes
Authorization: Bearer <token>
{
  "name": "My Home",
  "address": "123 Main St"
}
```

## Running

```bash
cd services/user-management
go mod download
go run main.go
```

Service starts on port **8086**.

## Environment Variables

- `POSTGRES_HOST`, `POSTGRES_PORT` - Database connection
- `JWT_SECRET` - Secret key for JWT (change in production!)
- `JWT_EXPIRY` - Token expiry (default: 24h)
- `USER_MANAGEMENT_PORT` - Service port (default: 8086)
