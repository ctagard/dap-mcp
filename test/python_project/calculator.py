"""A simple calculator module for testing the debugger."""

def add(a: int, b: int) -> int:
    """Add two numbers."""
    result = a + b
    return result


def subtract(a: int, b: int) -> int:
    """Subtract b from a."""
    result = a - b
    return result


def multiply(a: int, b: int) -> int:
    """Multiply two numbers."""
    result = a * b
    return result


def divide(a: int, b: int) -> float:
    """Divide a by b."""
    if b == 0:
        raise ValueError("Cannot divide by zero")
    result = a / b
    return result


def factorial(n: int) -> int:
    """Calculate factorial of n."""
    if n < 0:
        raise ValueError("Factorial not defined for negative numbers")
    if n <= 1:
        return 1
    result = 1
    for i in range(2, n + 1):
        result *= i
    return result


def fibonacci(n: int) -> list:
    """Generate first n Fibonacci numbers."""
    if n <= 0:
        return []
    if n == 1:
        return [0]

    fib = [0, 1]
    for i in range(2, n):
        next_val = fib[i-1] + fib[i-2]
        fib.append(next_val)
    return fib


class Calculator:
    """A calculator class with memory."""

    def __init__(self):
        self.memory = 0
        self.history = []

    def calculate(self, operation: str, a: int, b: int = None) -> float:
        """Perform a calculation and store in history."""
        if operation == "add":
            result = add(a, b)
        elif operation == "subtract":
            result = subtract(a, b)
        elif operation == "multiply":
            result = multiply(a, b)
        elif operation == "divide":
            result = divide(a, b)
        elif operation == "factorial":
            result = factorial(a)
        else:
            raise ValueError(f"Unknown operation: {operation}")

        self.history.append({
            "operation": operation,
            "args": (a, b) if b is not None else (a,),
            "result": result
        })
        return result

    def store(self, value: float):
        """Store a value in memory."""
        self.memory = value

    def recall(self) -> float:
        """Recall the value from memory."""
        return self.memory

    def clear_history(self):
        """Clear calculation history."""
        self.history = []


def main():
    """Main function to demonstrate the calculator."""
    print("Calculator Demo")
    print("=" * 40)

    # Basic operations
    print(f"5 + 3 = {add(5, 3)}")
    print(f"10 - 4 = {subtract(10, 4)}")
    print(f"6 * 7 = {multiply(6, 7)}")
    print(f"20 / 4 = {divide(20, 4)}")

    # Factorial
    print(f"5! = {factorial(5)}")

    # Fibonacci
    fib_10 = fibonacci(10)
    print(f"First 10 Fibonacci: {fib_10}")

    # Using the Calculator class
    calc = Calculator()
    calc.calculate("add", 10, 20)
    calc.calculate("multiply", 5, 5)
    calc.calculate("factorial", 6)

    print(f"\nCalculator history: {calc.history}")

    # Store and recall
    calc.store(42)
    print(f"Memory value: {calc.recall()}")

    print("\nDone!")


if __name__ == "__main__":
    main()
