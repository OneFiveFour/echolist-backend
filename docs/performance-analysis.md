# Performance Analysis

## Overview

This document analyzes the performance characteristics of the echolist-backend service and provides recommendations for optimization while maintaining the text-only (no database) architecture.

## Current Architecture Strengths

The backend is well-designed for a text-based system with several good practices:

- **Atomic writes** with temp file + rename (crash-safe)
- **Per-path locking** for concurrency control
- **HTTP/2 support** with reasonable timeouts
- **Structured logging** with minimal overhead
- **No unnecessary middleware** bloat
- **Clean separation** of concerns

## Identified Performance Bottleneck

### ListFiles Operation (file/list_files.go)

The primary performance concern is in the `ListFiles` endpoint, which performs extensive I/O operations:

**For every directory listing:**
1. `os.ReadDir(parentDir)` - reads the parent directory
2. **For EACH folder child:** `os.ReadDir(absPath)` again to count children
3. **For EACH note:** `os.ReadFile()` to generate preview (first 100 chars)
4. **For EACH task list:** `os.ReadFile()` + full parsing to count tasks

**Impact:** Listing a directory with 100 items could trigger 100+ file reads. On spinning disks or network filesystems, this becomes expensive.

### When This Matters

- **Small datasets (< 100 files):** Likely not noticeable due to OS page cache
- **Medium datasets (100-1000 files):** Noticeable latency on repeated listings
- **Large datasets (> 1000 files):** Significant performance degradation
- **Network filesystems:** Amplified latency on every file operation

## Recommended Solutions

### Option 1: Reverse Proxy with Caching (Recommended)

**Effort:** Low | **Impact:** High | **Code Changes:** None

Add a reverse proxy (Nginx or Caddy) in front of the service to cache responses.

#### Implementation with Nginx

Add to `docker-compose.yml`:

```yaml
services:
  nginx:
    image: nginx:alpine
    ports:
      - "9090:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - echolist-backend
    restart: unless-stopped

  echolist-backend:
    # ... existing config ...
    ports:
      - "8080:8080"  # Only expose internally
```

Create `nginx.conf`:

```nginx
events {
    worker_connections 1024;
}

http {
    proxy_cache_path /var/cache/nginx levels=1:2 keys_zone=api_cache:10m max_size=100m inactive=60m;
    
    upstream backend {
        server echolist-backend:8080;
    }
    
    server {
        listen 80;
        
        location / {
            proxy_pass http://backend;
            proxy_cache api_cache;
            proxy_cache_valid 200 1m;  # Cache successful responses for 1 minute
            proxy_cache_key "$request_uri|$http_authorization";
            proxy_cache_methods GET HEAD;
            add_header X-Cache-Status $upstream_cache_status;
            
            # Forward headers
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }
        
        # Don't cache auth endpoints
        location ~ ^/auth\.v1\.AuthService/ {
            proxy_pass http://backend;
            proxy_no_cache 1;
            proxy_cache_bypass 1;
        }
    }
}
```

**Pros:**
- Zero code changes required
- Automatic cache invalidation via TTL
- Reduces backend load significantly
- Industry-standard solution

**Cons:**
- Stale data for cache duration (configurable, 30-60s recommended)
- Additional container to manage

#### Alternative: Caddy (Simpler Configuration)

Create `Caddyfile`:

```
:80 {
    reverse_proxy echolist-backend:8080 {
        header_up Host {host}
        header_up X-Real-IP {remote}
    }
    
    @cacheable {
        not path /auth.v1.AuthService/*
        method GET HEAD
    }
    
    cache @cacheable {
        ttl 1m
    }
}
```

### Option 2: Lazy Loading (Medium Effort, High Impact)

**Effort:** Medium | **Impact:** High | **Code Changes:** Backend + Frontend

Modify `ListFiles` to return minimal data and add separate endpoints for detailed information.

#### Changes Required

**Backend:**
- Remove preview generation from `buildNoteEntry`
- Remove child counting from `buildFolderEntry`
- Remove task parsing from `buildTaskListEntry`
- Add new endpoints:
  - `GetFilePreview(path)` - returns preview for a single file
  - `GetFolderStats(path)` - returns child count for a folder
  - `GetTaskListStats(path)` - returns task counts

**Frontend:**
- Call `ListFiles` for initial directory listing (fast)
- Call detail endpoints on-demand (hover, expand, etc.)

**Pros:**
- Dramatically faster listings
- Clients control what data they need
- Better scalability

**Cons:**
- Requires frontend changes
- More API calls (but smaller, targeted ones)

### Option 3: OS-Level Caching (Free)

**Effort:** Minimal | **Impact:** Low-Medium | **Code Changes:** None

Linux already caches file metadata and content in the page cache. Optimize Docker volume mounts:

```yaml
# docker-compose.yml
volumes:
  - ./data:/app/data:cached  # On Mac/Windows, improves performance
```

For production on Linux, ensure sufficient memory for page cache:
- Monitor with `free -h` and `vmstat`
- The OS will automatically cache frequently accessed files

**Pros:**
- Free, automatic
- No code changes
- Works well for repeated access patterns

**Cons:**
- Limited impact on first access
- Doesn't help with the fundamental I/O volume issue

### Option 4: In-Memory Metadata Cache (Higher Effort)

**Effort:** High | **Impact:** High | **Code Changes:** Significant

Implement an in-memory cache for file metadata with filesystem watching.

#### Pseudo-Implementation

```go
type MetadataCache struct {
    mu      sync.RWMutex
    entries map[string]CachedEntry
    watcher *fsnotify.Watcher
}

type CachedEntry struct {
    Preview    string
    ChildCount int32
    TaskStats  TaskStats
    ModTime    time.Time
}

// Invalidate on writes (leverage existing path locks)
// Use fsnotify to watch for external changes
```

**Pros:**
- Very fast reads
- Still maintains text-based storage
- Fine-grained cache invalidation

**Cons:**
- Significant complexity
- Cache invalidation is hard (external file changes)
- Memory usage grows with dataset
- Requires careful synchronization

## Recommendation

**Start with Option 1 (Reverse Proxy Caching)**

This provides the best effort-to-impact ratio:
- No code changes required
- Industry-standard solution
- Easy to configure and tune
- For personal/small-team apps, 30-60 second cache TTL is acceptable

**Then evaluate Option 2 (Lazy Loading)** if:
- You have > 1000 files per directory
- Users complain about slow listings
- You want better architectural patterns

**Skip Option 4** unless:
- You have very specific performance requirements
- You're willing to maintain complex caching logic
- Your dataset is large but fits in memory

## Performance Testing

To measure the impact, use these tools:

```bash
# Benchmark a list operation
time grpcurl -plaintext -d '{"parent_dir":""}' localhost:8080 file.v1.FileService/ListFiles

# Load testing with ghz
ghz --insecure --proto proto/file/v1/file.proto \
    --call file.v1.FileService.ListFiles \
    -d '{"parent_dir":""}' \
    -n 100 -c 10 \
    localhost:8080

# Monitor file I/O
iostat -x 1

# Monitor cache hit rates (with Nginx)
curl -s http://localhost:9090/some-endpoint -I | grep X-Cache-Status
```

## Conclusion

The backend is well-architected for a text-based system. The identified bottleneck is a common pattern in file-based systems and has well-established solutions. Starting with reverse proxy caching provides immediate benefits with minimal effort while maintaining the simplicity of the text-only approach.
