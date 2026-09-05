package utils

import (
	"testing"
)

func TestIsValidLuhn(t *testing.T) {
	tests := []struct {
		name     string
		number   string
		expected bool
	}{
		// Валидные номера
		{
			name:     "valid number 1",
			number:   "12345678903",
			expected: true,
		},
		{
			name:     "valid number 2",
			number:   "5555555555554444",
			expected: true,
		},
		{
			name:     "valid number 3",
			number:   "4111111111111111",
			expected: true,
		},
		{
			name:     "valid number 4",
			number:   "49927398716",
			expected: true,
		},
		{
			name:     "valid number with spaces",
			number:   "1234 5678 903",
			expected: true,
		},
		{
			name:     "valid number with dashes",
			number:   "1234-5678-903",
			expected: true,
		},

		// Невалидные номера
		{
			name:     "invalid number 1",
			number:   "1234567890",
			expected: false,
		},
		{
			name:     "invalid number 2",
			number:   "12345678901",
			expected: false,
		},
		{
			name:     "invalid number 3",
			number:   "49927398717",
			expected: false,
		},
		{
			name:     "invalid number with letters",
			number:   "1234abcd",
			expected: false,
		},
		{
			name:     "empty string",
			number:   "",
			expected: false,
		},
		{
			name:     "only spaces",
			number:   "   ",
			expected: false,
		},
		{
			name:     "special characters",
			number:   "1234!@#$",
			expected: false,
		},

		// Крайние случаи
		{
			name:     "single digit valid",
			number:   "0",
			expected: true,
		},
		{
			name:     "single digit invalid",
			number:   "1",
			expected: false,
		},
		{
			name:     "number with leading zeros",
			number:   "0012345678903",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidLuhn(tt.number)
			if result != tt.expected {
				t.Errorf("IsValidLuhn(%q) = %v, expected %v", tt.number, result, tt.expected)
			}
		})
	}
}

func TestNormalizeOrderNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already normalized",
			input:    "12345678903",
			expected: "12345678903",
		},
		{
			name:     "with spaces",
			input:    "1234 5678 903",
			expected: "12345678903",
		},
		{
			name:     "with dashes",
			input:    "1234-5678-903",
			expected: "12345678903",
		},
		{
			name:     "with spaces and dashes",
			input:    "1234 - 5678 - 903",
			expected: "12345678903",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: "",
		},
		{
			name:     "with leading/trailing spaces",
			input:    "  12345678903  ",
			expected: "12345678903",
		},
		{
			name:     "with multiple spaces",
			input:    "1234  5678  903",
			expected: "12345678903",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeOrderNumber(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeOrderNumber(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateAndNormalizeOrderNumber(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		shouldError bool
		errorType   error
	}{
		{
			name:        "valid number",
			input:       "12345678903",
			expected:    "12345678903",
			shouldError: false,
		},
		{
			name:        "valid number with spaces",
			input:       "1234 5678 903",
			expected:    "12345678903",
			shouldError: false,
		},
		{
			name:        "empty string",
			input:       "",
			expected:    "",
			shouldError: true,
			errorType:   ErrEmptyOrderNumber,
		},
		{
			name:        "only spaces",
			input:       "   ",
			expected:    "",
			shouldError: true,
			errorType:   ErrEmptyOrderNumber,
		},
		{
			name:        "invalid number",
			input:       "1234567890",
			expected:    "",
			shouldError: true,
			errorType:   ErrInvalidOrderNumber,
		},
		{
			name:        "with letters",
			input:       "1234abcd",
			expected:    "",
			shouldError: true,
			errorType:   ErrInvalidOrderNumber,
		},
		{
			name:        "special characters",
			input:       "1234!@#$",
			expected:    "",
			shouldError: true,
			errorType:   ErrInvalidOrderNumber,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateAndNormalizeOrderNumber(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got nil")
				}
				if tt.errorType != nil && err != tt.errorType {
					t.Errorf("expected error %v, got %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

// Бенчмарки
func BenchmarkIsValidLuhn(b *testing.B) {
	number := "12345678903"
	for i := 0; i < b.N; i++ {
		IsValidLuhn(number)
	}
}

func BenchmarkIsValidLuhnLong(b *testing.B) {
	number := "4111111111111111"
	for i := 0; i < b.N; i++ {
		IsValidLuhn(number)
	}
}

func BenchmarkNormalizeOrderNumber(b *testing.B) {
	number := "1234 5678 903"
	for i := 0; i < b.N; i++ {
		NormalizeOrderNumber(number)
	}
}
