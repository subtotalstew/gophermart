// Package middleware provides HTTP middleware components.
//
// The package implements:
//   - Authentication middleware (JWT validation)
//   - Logging middleware (request/response logging)
//   - Compression middleware (gzip support)
//
// Middleware components are used to:
//   - Protect routes that require authentication
//   - Log all HTTP requests
//   - Compress responses when supported by client
package middleware
