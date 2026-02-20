# Implementation Plan: JWT Authentication

## Overview

Add JWT-based authentication to the ConnectRPC notes backend. Implementation proceeds bottom-up: protobuf definitions → core auth components (UserStore, TokenService) → interceptor → AuthServer RPC handlers → main.go wiring → Docker config updates.

## Tasks

- [x] 1. Define AuthService protobuf and generate Go code
  - [x] 1.1 Create `proto/auth/v1/auth.proto` with LoginRequest/LoginResponse, RefreshTokenRequest/RefreshTokenResponse messages and AuthService with Login + RefreshToken RPCs
    - Update `proto/buf.yaml` and `proto/buf.gen.yaml` if needed to include the new `auth/v1` path
    - Run `buf generate` to produce Go code in `proto/gen/auth/v1/`
    - _Requirements: 6.1, 2.4, 4.4_

- [-] 2. Implement UserStore
  - [x] 2.1 Create `auth/user_store.go` with User struct, UserStore struct, NewUserStore, LoadOrInitialize, Authenticate, and getUser methods
    - Store users as JSON array in a file at the configured path
    - Use bcrypt with cost >= 10 for password hashing
    - Use sync.RWMutex for concurrent-safe access
    - LoadOrInitialize creates default admin user from env vars if file doesn't exist
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

  - [x] 2.2 Write property tests for UserStore (user_store_test.go)
    - **Property 1: User store round-trip**
    - **Validates: Requirements 1.1, 1.5**
    - **Property 2: Bcrypt hashing invariant**
    - **Validates: Requirements 1.2, 1.4**

  - [x] 2.3 Write unit tests for UserStore edge cases
    - Test default user creation when file doesn't exist
    - Test loading existing valid users.json
    - Test error on malformed JSON file
    - Test Authenticate with wrong password returns error
    - Test getUser with non-existent username returns not-found error
    - _Requirements: 1.3, 1.5_

- [x] 3. Implement TokenService
  - [x] 3.1 Create `auth/token_service.go` with TokenClaims struct, TokenService struct, NewTokenService, GenerateAccessToken, GenerateRefreshToken, and ValidateToken methods
    - Use golang-jwt/jwt/v5 with HS256 signing
    - Include username, sub, iat, exp claims
    - Configurable TTL for access and refresh tokens
    - _Requirements: 2.5, 2.6_

  - [x] 3.2 Write property tests for TokenService (token_service_test.go)
    - **Property 3: Token generation round-trip with claims**
    - **Validates: Requirements 2.5, 2.6**

  - [x] 3.3 Write unit tests for TokenService edge cases
    - Test expired token returns error
    - Test token signed with wrong secret returns error
    - _Requirements: 3.2, 3.3_

- [x] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Implement Auth Interceptor
  - [x] 5.1 Create `auth/interceptor.go` with NewAuthInterceptor function and publicProcedures map
    - Check procedure against publicProcedures allowlist
    - Extract and validate Bearer token from Authorization header
    - Return codes.InvalidArgument for malformed header, codes.Unauthenticated for missing/invalid/expired tokens
    - Inject username into context on success
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

  - [x] 5.2 Write property tests for Auth Interceptor (interceptor_test.go)
    - **Property 5: Valid token passes interceptor with correct context**
    - **Validates: Requirements 3.1, 3.6**
    - **Property 6: Invalid tokens rejected by interceptor**
    - **Validates: Requirements 3.3**
    - **Property 7: Public endpoints bypass authentication**
    - **Validates: Requirements 3.5**

  - [x] 5.3 Write unit tests for Auth Interceptor edge cases
    - Test missing Authorization header returns Unauthenticated
    - Test malformed header (no "Bearer " prefix) returns InvalidArgument
    - Test expired token returns Unauthenticated
    - _Requirements: 3.2, 3.3, 3.4_

- [x] 6. Implement AuthServer RPC handlers
  - [x] 6.1 Create `auth/auth_server.go` with AuthServer struct, NewAuthServer, Login, and RefreshToken handler methods
    - Login: validate credentials via UserStore.Authenticate, generate access + refresh tokens via TokenService
    - RefreshToken: validate refresh token via TokenService.ValidateToken, generate new access token
    - Return generic "invalid credentials" error for both bad username and bad password
    - _Requirements: 2.1, 2.2, 2.3, 4.1, 4.2, 4.3_

  - [x] 6.2 Write property tests for AuthServer (auth_server_test.go)
    - **Property 4: Valid credentials produce valid tokens**
    - **Validates: Requirements 2.1**
    - **Property 8: Valid refresh token produces new access token**
    - **Validates: Requirements 4.1**

  - [x] 6.3 Write unit tests for AuthServer edge cases
    - Test Login with invalid username returns Unauthenticated
    - Test Login with wrong password returns Unauthenticated
    - Test Login error messages are identical for bad username vs bad password
    - Test RefreshToken with expired token returns Unauthenticated
    - Test RefreshToken with malformed token returns Unauthenticated
    - _Requirements: 2.2, 2.3, 4.2, 4.3_

- [x] 7. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 8. Wire everything together in main.go
  - [ ] 8.1 Update `main.go` to initialize auth components and register handlers
    - Add JWT_SECRET validation (fatal if empty)
    - Parse ACCESS_TOKEN_EXPIRY_MINUTES and REFRESH_TOKEN_EXPIRY_MINUTES with defaults
    - Initialize UserStore with LoadOrInitialize using AUTH_DEFAULT_USER and AUTH_DEFAULT_PASSWORD
    - Initialize TokenService and Auth Interceptor
    - Register AuthService handler on the mux with interceptor
    - Add interceptor to existing NotesService handler
    - Add auth service to gRPC reflection
    - Log authentication status and token TTLs on startup
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 6.2, 6.3_

  - [ ] 8.2 Add helper functions `parseDurationMinutesEnv` and `envOrDefault` in main.go
    - _Requirements: 5.3, 5.4, 5.5_

- [ ] 9. Update Docker configuration
  - [ ] 9.1 Update `docker-compose.yml` to include JWT_SECRET, AUTH_DEFAULT_USER, AUTH_DEFAULT_PASSWORD, ACCESS_TOKEN_EXPIRY_MINUTES, and REFRESH_TOKEN_EXPIRY_MINUTES environment variables
    - _Requirements: 7.1_

  - [ ] 9.2 Update `go.mod` with new dependencies (`golang-jwt/jwt/v5`, `golang.org/x/crypto`, `pgregory.net/rapid`)
    - _Requirements: 7.2_

- [ ] 10. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Property tests use `pgregory.net/rapid` with minimum 100 iterations
- Unit tests use standard Go `testing` package
- Checkpoints ensure incremental validation
