// Package accrual provides a client for interacting with the external
// loyalty points calculation system (Accrual System).
//
// The package implements:
//   - HTTP client for making requests to the Accrual System API
//   - Handling of various response statuses (200, 204, 429, 500)
//   - Retry-After handling for rate limiting
//   - Status mapping between Accrual System and internal statuses
//
// Example usage:
//
//	client := accrual.NewClient("http://accrual-system:8080")
//	resp, err := client.GetOrderInfo(ctx, "12345678903")
//	if err != nil {
//	    // handle error
//	}
//	// process response
package accrual
