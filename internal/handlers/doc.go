// Package handlers implements HTTP handlers for the Gophermart API.
//
// The package provides handlers for:
//   - User registration and authentication
//   - Order management (upload, list)
//   - Balance management (get, withdraw)
//   - Withdrawal history
//
// Each handler is responsible for:
//   - Request validation
//   - Calling appropriate service methods
//   - Response formatting
//   - Error handling with appropriate HTTP status codes
package handlers
