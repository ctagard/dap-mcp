package main

import "fmt"

// Basic arithmetic functions
func add(a, b int) int {
	result := a + b
	return result
}

func subtract(a, b int) int {
	result := a - b
	return result
}

func multiply(a, b int) int {
	result := a * b
	return result
}

func divide(a, b int) (int, error) {
	if b == 0 {
		return 0, fmt.Errorf("cannot divide by zero")
	}
	result := a / b
	return result, nil
}

func factorial(n int) int {
	if n < 0 {
		return -1 // Error case
	}
	if n <= 1 {
		return 1
	}
	result := 1
	for i := 2; i <= n; i++ {
		result *= i
	}
	return result
}

func fibonacci(n int) []int {
	if n <= 0 {
		return []int{}
	}
	if n == 1 {
		return []int{0}
	}

	fib := make([]int, n)
	fib[0] = 0
	fib[1] = 1
	for i := 2; i < n; i++ {
		fib[i] = fib[i-1] + fib[i-2]
	}
	return fib
}

// HistoryEntry represents a calculation history entry
type HistoryEntry struct {
	Operation string
	Args      []int
	Result    int
}

// Calculator with memory and history
type Calculator struct {
	memory  int
	history []HistoryEntry
}

func NewCalculator() *Calculator {
	return &Calculator{
		memory:  0,
		history: make([]HistoryEntry, 0),
	}
}

func (c *Calculator) Calculate(operation string, a, b int) (int, error) {
	var result int
	var err error

	switch operation {
	case "add":
		result = add(a, b)
	case "subtract":
		result = subtract(a, b)
	case "multiply":
		result = multiply(a, b)
	case "divide":
		result, err = divide(a, b)
		if err != nil {
			return 0, err
		}
	case "factorial":
		result = factorial(a)
	default:
		return 0, fmt.Errorf("unknown operation: %s", operation)
	}

	c.history = append(c.history, HistoryEntry{
		Operation: operation,
		Args:      []int{a, b},
		Result:    result,
	})

	return result, nil
}

func (c *Calculator) Store(value int) {
	c.memory = value
}

func (c *Calculator) Recall() int {
	return c.memory
}

func (c *Calculator) GetHistory() []HistoryEntry {
	return c.history
}

func (c *Calculator) ClearHistory() {
	c.history = make([]HistoryEntry, 0)
}

func main() {
	fmt.Println("Calculator Demo (Go)")
	fmt.Println("========================================")

	// Basic operations
	fmt.Printf("5 + 3 = %d\n", add(5, 3))
	fmt.Printf("10 - 4 = %d\n", subtract(10, 4))
	fmt.Printf("6 * 7 = %d\n", multiply(6, 7))
	result, _ := divide(20, 4)
	fmt.Printf("20 / 4 = %d\n", result)

	// Factorial
	fmt.Printf("5! = %d\n", factorial(5))

	// Fibonacci
	fib10 := fibonacci(10)
	fmt.Printf("First 10 Fibonacci: %v\n", fib10)

	// Using the Calculator
	calc := NewCalculator()
	calc.Calculate("add", 10, 20)
	calc.Calculate("multiply", 5, 5)
	calc.Calculate("factorial", 6, 0)

	fmt.Printf("\nCalculator history: %+v\n", calc.GetHistory())

	// Store and recall
	calc.Store(42)
	fmt.Printf("Memory value: %d\n", calc.Recall())

	fmt.Println("\nDone!")
}
