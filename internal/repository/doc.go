// Package repository provides data access layer for the application.
//
// The package implements:
//   - PostgreSQL database operations
//   - CRUD operations for users, orders, balances, and withdrawals
//   - Transaction support
//   - Query building and execution
//
// Each repository handles:
//   - Database connections and pooling
//   - SQL query execution
//   - Error handling and conversion
//   - Data mapping between database and domain models
package repository
