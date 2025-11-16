# Flight History Monorepo

A high-performance, memory-optimized monorepo for flight tracking applications built with Go. This project demonstrates advanced memory optimization techniques and provides a shared library for efficient resource management.

## 🏗️ Architecture

```
fo-misc/
├── libs/
│   └── optimizations/              # Shared memory optimization library
│       ├── cache.go         # Optimized caching with Ristretto
│       ├── database.go      # MongoDB cursor and connection optimizations
│       ├── structures.go    # Memory-efficient data structures
│       ├── monitoring.go    # Memory monitoring and GC optimization
│       └── utils.go         # Utility functions and helpers
├── apps/
│   ├── customer-backend/    # Customer-facing API server
│   │   ├── main_optimized.go
│   │   └── ... (existing files)
│   └── admin-backend/       # Admin API server
│       ├── main_optimized.go
│       └── ... (existing files)
├── go.work                  # Go workspace configuration
├── Makefile                 # Build and development commands
└── README.md
```

## 🛠️ Quick Start

### Prerequisites
- Go 1.25.0 or later
- MongoDB instance
- AWS credentials (for S3 operations)

### Installation

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd fo-misc
   ```

2. **Install dependencies**:
   ```bash
   make install
   ```

3. **Set up environment variables**:
   ```bash
   cp apps/customer-backend/.env.example apps/customer-backend/.env
   cp apps/admin-backend/.env.example apps/admin-backend/.env
   # Edit the .env files with your configuration
   ```

### Development

**Start the customer backend**:
```bash
make dev-customer
```

**Start the admin backend**:
```bash
make dev-admin
```

**Run both with memory profiling**:
```bash
make profile-customer  # Terminal 1
make profile-admin     # Terminal 2
```

### Building

**Build all components**:
```bash
make build
```

**Build for production**:
```bash
make build-prod
```

**Build Docker images**:
```bash
make docker-build
```

## 🧪 Testing

**Run all tests**:
```bash
make test
```

**Run benchmarks**:
```bash
make benchmark
```

**Memory analysis**:
```bash
make memory-analysis
```


## ⚙️ Configuration

### Environment Variables

**Memory Optimization Settings**:
```bash
GOGC=75                    # More aggressive garbage collection
GOMEMLIMIT=1GB            # Memory limit (Go 1.19+)
GOMAXPROCS=4              # CPU core limit

# MongoDB Optimization
MONGODB_MAX_POOL_SIZE=25   # Connection pool size
MONGODB_MIN_POOL_SIZE=5    # Minimum connections
MONGODB_MAX_IDLE_TIME=5m   # Connection idle timeout
```

### Cache Configuration

```go
config := common.CacheConfig{
    NumCounters: 1e4,              // Track 10k keys
    MaxCost:     1e6,              // 1MB cache size
    BufferItems: 64,               // Buffer size
    DefaultTTL:  15 * time.Minute, // Default expiration
}
```


## 🔧 Development Commands

```bash
# Development
make dev-customer          # Start customer backend
make dev-admin            # Start admin backend
make test                 # Run all tests
make benchmark            # Run performance benchmarks

# Building
make build                # Build all components
make build-prod           # Production build with optimizations
make docker-build         # Build Docker images

# Code Quality
make lint                 # Run linter
make format               # Format code
make memory-analysis      # Analyze memory allocations

# Monitoring
make profile-customer     # Start with memory profiling
make load-test-customer   # Run load tests
```

## 🙏 Acknowledgments

- **Ristretto Cache** - High-performance caching library
- **MongoDB Go Driver** - Efficient database operations
- **Gin Framework** - Fast HTTP router
- **Go Team** - Excellent memory management tools

---

**Built with ❤️ and optimized for performance**
