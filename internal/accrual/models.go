package accrual

// AccrualStatus - статусы заказа в системе расчета баллов
const (
	StatusRegistered = "REGISTERED"
	StatusInvalid    = "INVALID"
	StatusProcessing = "PROCESSING"
	StatusProcessed  = "PROCESSED"
)

// OrderResponse - ответ от системы расчета баллов
type OrderResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

// ErrorResponse - ошибка от системы расчета баллов
type ErrorResponse struct {
	Error string `json:"error,omitempty"`
}

// Mapping статусов из Accrual System в статусы нашего сервиса
var StatusMapping = map[string]string{
	StatusRegistered: "NEW",
	StatusInvalid:    "INVALID",
	StatusProcessing: "PROCESSING",
	StatusProcessed:  "PROCESSED",
}

// MapAccrualStatusToOrderStatus преобразует статус из Accrual System в статус заказа
func MapAccrualStatusToOrderStatus(accrualStatus string) string {
	if mapped, ok := StatusMapping[accrualStatus]; ok {
		return mapped
	}
	return "NEW"
}
