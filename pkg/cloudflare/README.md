# Cloudflare Cache Utilities

This package provides a lightweight helper for purging cached assets from Cloudflare using API tokens.

## Installation

```bash
go get github.com/infigaming-com/go-common/pkg/cloudflare
```

## Usage

```go
import (
    "context"
    "log"

    "github.com/infigaming-com/go-common/pkg/cloudflare"
)

func main() {
    err := cloudflare.PurgeCloudflareCache(context.Background(), "<API_TOKEN>", "<ZONE_ID>", []string{
        "https://example.com/file1.js",
        "https://example.com/file2.css",
    })
    if err != nil {
        log.Fatalf("failed to purge cache: %v", err)
    }
    log.Println("cache purge successful")
}
```

## Testing

```bash
go test ./pkg/cloudflare -v
```

## Notes

- Uses Bearer token authentication to access the Cloudflare API.
- Exposes a stateless helper that requires no additional configuration.
- Includes one retry for transient network failures.
