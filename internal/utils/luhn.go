package utils

import (
	"strings"
)

func IsValidLuhn(number string) bool {
	// Удаляем пробелы и другие разделители
	number = strings.TrimSpace(number)
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")

	// Проверяем, что строка состоит только из цифр
	if number == "" {
		return false
	}

	// Проверяем, что все символы - цифры
	for _, ch := range number {
		if ch < '0' || ch > '9' {
			return false
		}
	}

	// Преобразуем строку в массив цифр
	digits := make([]int, len(number))
	for i, ch := range number {
		digits[i] = int(ch - '0')
	}

	// Алгоритм Луна
	sum := 0
	parity := len(digits) % 2

	for i, digit := range digits {
		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}

	return sum%10 == 0
}

// NormalizeOrderNumber приводит номер заказа к стандартному виду
func NormalizeOrderNumber(number string) string {
	number = strings.TrimSpace(number)
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")
	return number
}

// ValidateAndNormalizeOrderNumber проверяет и нормализует номер заказа
func ValidateAndNormalizeOrderNumber(number string) (string, error) {
	normalized := NormalizeOrderNumber(number)

	if normalized == "" {
		return "", ErrEmptyOrderNumber
	}

	if !IsValidLuhn(normalized) {
		return "", ErrInvalidOrderNumber
	}

	return normalized, nil
}

// Ошибки
var (
	ErrEmptyOrderNumber   = &OrderNumberError{Message: "order number cannot be empty"}
	ErrInvalidOrderNumber = &OrderNumberError{Message: "invalid order number format"}
)

type OrderNumberError struct {
	Message string
}

func (e *OrderNumberError) Error() string {
	return e.Message
}
