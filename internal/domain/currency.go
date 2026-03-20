package domain

import "fmt"

type CurrencyCode string

const (
	USD CurrencyCode = "USD"
	EUR CurrencyCode = "EUR"
	GBP CurrencyCode = "GBP"
	JPY CurrencyCode = "JPY"
	CNY CurrencyCode = "CNY"
)

func (c CurrencyCode) IsValid() bool {
	if len(c) != 3 {
		return false
	}

	for _, ch := range c {
		if ch < 'A' || ch > 'Z' {
			return false
		}
	}

	return true
}

func (c CurrencyCode) String() string {
	return string(c)
}

func ValidateCurrency(code string, supported []string) error {
	currency := CurrencyCode(code)
	if !currency.IsValid() {
		return fmt.Errorf("invalid currency code format: %s (must be 3 uppercase letters)", code)
	}

	for _, supportedCode := range supported {
		if supportedCode == code {
			return nil
		}
	}

	return fmt.Errorf("unsupported currency code: %s", code)
}
