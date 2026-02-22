# Requirements Document

## Introduction

This feature adds JWT-based authentication to the existing Go ConnectRPC notes backend service. The service is being exposed to the internet via a subdomain routed through Pangolin, and currently has no authentication. The solution uses username/password login with bcrypt password hashing and JWT tokens for session management, implemented as a Connect interceptor. The user store is file-based, suitable for a single-user or small-team self-hosted deployment.

## Glossary

- **Auth_Service**: The authentication component responsible for user login, token generation, and token validation
- **User_Store**: A file-based storage mechanism (JSON file) that holds user credentials (username and bcrypt-hashed password)
- **JWT_Token**: A JSON Web Token used to authenticate requests after login; contains claims such as username and expiration time
- **Access_Token**: A short-lived JWT_Token (configurable expiry, default 15 minutes) used to authorize individual requests
- **Refresh_Token**: A longer-lived JWT_Token (configurable expiry, default 7 days) used to obtain new Access_Tokens without re-entering credentials
- **Auth_Interceptor**: A Connect interceptor that validates JWT_Tokens on incoming requests before they reach RPC handlers
- **Protected_Endpoint**: Any NotesService RPC endpoint that requires a valid Access_Token to be called
- **Public_Endpoint**: An RPC endpoint that does not require authentication (Login, RefreshToken)
- **JWT_Secret**: A secret key used to sign and verify JWT_Tokens, provided via environment variable

## Requirements

### Requirement 1: User Credential Storage

**User Story:** As a system administrator, I want user credentials stored securely in a file-based store, so that I can manage a small number of users without needing a database.

#### Acceptance Criteria

1. THE User_Store SHALL store user credentials as a JSON file containing username and bcrypt-hashed password pairs
2. WHEN a new user record is created, THE User_Store SHALL hash the password using bcrypt with a minimum cost factor of 10 before storing
3. WHEN the User_Store file does not exist at startup, THE Auth_Service SHALL create a default admin user using credentials from environment variables (AUTH_DEFAULT_USER, AUTH_DEFAULT_PASSWORD)
4. THE User_Store SHALL reject any attempt to store a password in plaintext by requiring bcrypt hashing for all password values
5. WHEN a username lookup is performed, THE User_Store SHALL return the stored credentials if the username exists, or return a "not found" error if the username does not exist

### Requirement 2: User Login

**User Story:** As a user, I want to log in with my username and password, so that I can receive tokens to access the notes service.

#### Acceptance Criteria

1. WHEN a user submits valid credentials to the Login endpoint, THE Auth_Service SHALL return an Access_Token and a Refresh_Token
2. WHEN a user submits an invalid username, THE Auth_Service SHALL return an "unauthenticated" error without revealing whether the username or password was incorrect
3. WHEN a user submits an invalid password for a valid username, THE Auth_Service SHALL return an "unauthenticated" error without revealing whether the username or password was incorrect
4. THE Auth_Service SHALL define the Login endpoint as a new RPC method in the AuthService protobuf definition
5. THE Auth_Service SHALL include the username and token expiration time as claims in the generated Access_Token
6. THE Auth_Service SHALL sign all generated JWT_Tokens using the JWT_Secret

### Requirement 3: Token Validation and Protected Endpoints

**User Story:** As a user, I want my requests to be authenticated via JWT tokens, so that only authorized users can access the notes service.

#### Acceptance Criteria

1. WHEN a request to a Protected_Endpoint includes a valid Access_Token in the Authorization header (Bearer scheme), THE Auth_Interceptor SHALL allow the request to proceed to the RPC handler
2. WHEN a request to a Protected_Endpoint includes an expired Access_Token, THE Auth_Interceptor SHALL return an "unauthenticated" error
3. WHEN a request to a Protected_Endpoint includes a malformed or tampered Access_Token, THE Auth_Interceptor SHALL return an "unauthenticated" error
4. WHEN a request to a Protected_Endpoint does not include an Authorization header, THE Auth_Interceptor SHALL return an "unauthenticated" error
5. THE Auth_Interceptor SHALL allow requests to Public_Endpoints (Login, RefreshToken) without requiring an Access_Token
6. THE Auth_Interceptor SHALL extract the username from a valid Access_Token and make it available in the request context

### Requirement 4: Token Refresh

**User Story:** As a user, I want to refresh my access token without re-entering my credentials, so that I can maintain a seamless session.

#### Acceptance Criteria

1. WHEN a valid Refresh_Token is submitted to the RefreshToken endpoint, THE Auth_Service SHALL return a new Access_Token
2. WHEN an expired Refresh_Token is submitted, THE Auth_Service SHALL return an "unauthenticated" error
3. WHEN a malformed or tampered Refresh_Token is submitted, THE Auth_Service SHALL return an "unauthenticated" error
4. THE Auth_Service SHALL define the RefreshToken endpoint as a new RPC method in the AuthService protobuf definition

### Requirement 5: Configuration

**User Story:** As a system administrator, I want to configure authentication parameters via environment variables, so that I can adjust security settings without modifying code.

#### Acceptance Criteria

1. THE Auth_Service SHALL read the JWT signing secret from the JWT_SECRET environment variable
2. WHEN the JWT_SECRET environment variable is not set or is empty, THE Auth_Service SHALL refuse to start and log a descriptive error message
3. THE Auth_Service SHALL read the Access_Token expiry duration from the ACCESS_TOKEN_EXPIRY_MINUTES environment variable (value in minutes), defaulting to 15 minutes if not set
4. THE Auth_Service SHALL read the Refresh_Token expiry duration from the REFRESH_TOKEN_EXPIRY_MINUTES environment variable (value in minutes), defaulting to 10080 minutes (7 days) if not set
5. THE Auth_Service SHALL read the default admin username from the AUTH_DEFAULT_USER environment variable, defaulting to "admin" if not set
6. THE Auth_Service SHALL read the default admin password from the AUTH_DEFAULT_PASSWORD environment variable

### Requirement 6: Protobuf and Service Definition

**User Story:** As a developer, I want authentication defined as a separate protobuf service, so that auth concerns are cleanly separated from notes functionality.

#### Acceptance Criteria

1. THE Auth_Service SHALL be defined in a separate protobuf file (auth/v1/auth.proto) with Login and RefreshToken RPC methods
2. THE Auth_Service SHALL register its Connect handler on the same HTTP mux as the existing NotesService
3. WHEN the server starts, THE Auth_Service SHALL log that authentication is enabled and the configured token expiry durations

### Requirement 7: Docker and Deployment Integration

**User Story:** As a system administrator, I want the authentication feature to integrate with the existing Docker Compose deployment, so that I can deploy securely with minimal configuration changes.

#### Acceptance Criteria

1. THE docker-compose.yml SHALL include the JWT_SECRET, AUTH_DEFAULT_USER, and AUTH_DEFAULT_PASSWORD as environment variables for the echolist-backend service
2. THE Dockerfile SHALL continue to produce a single binary that includes the authentication functionality without additional runtime dependencies
3. WHEN the container starts with AUTH_DEFAULT_PASSWORD set, THE Auth_Service SHALL create the default admin user on first run if the User_Store file does not already exist
