// Package worker provides background order processing.
//
// The package implements:
//   - OrderProcessor: background worker that processes orders
//   - Periodic polling of orders with NEW and PROCESSING status
//   - Integration with Accrual System for order status updates
//   - Balance updates based on accrual results
//
// The worker runs as a goroutine and:
//   - Queries orders requiring processing
//   - Fetches order information from Accrual System
//   - Updates order statuses and accruals
//   - Adds accruals to user balances
package worker
