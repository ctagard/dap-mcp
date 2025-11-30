"use strict";
// A simple calculator module for testing the TypeScript debugger
function add(a, b) {
    const result = a + b;
    return result;
}
function subtract(a, b) {
    const result = a - b;
    return result;
}
function multiply(a, b) {
    const result = a * b;
    return result;
}
function divide(a, b) {
    if (b === 0) {
        throw new Error("Cannot divide by zero");
    }
    const result = a / b;
    return result;
}
function factorial(n) {
    if (n < 0) {
        throw new Error("Factorial not defined for negative numbers");
    }
    if (n <= 1) {
        return 1;
    }
    let result = 1;
    for (let i = 2; i <= n; i++) {
        result *= i;
    }
    return result;
}
function fibonacci(n) {
    if (n <= 0) {
        return [];
    }
    if (n === 1) {
        return [0];
    }
    const fib = [0, 1];
    for (let i = 2; i < n; i++) {
        const nextVal = fib[i - 1] + fib[i - 2];
        fib.push(nextVal);
    }
    return fib;
}
class Calculator {
    constructor() {
        this.memory = 0;
        this.history = [];
    }
    calculate(operation, a, b) {
        let result;
        switch (operation) {
            case "add":
                result = add(a, b);
                break;
            case "subtract":
                result = subtract(a, b);
                break;
            case "multiply":
                result = multiply(a, b);
                break;
            case "divide":
                result = divide(a, b);
                break;
            case "factorial":
                result = factorial(a);
                break;
            default:
                throw new Error(`Unknown operation: ${operation}`);
        }
        this.history.push({
            operation,
            args: b !== undefined ? [a, b] : [a],
            result
        });
        return result;
    }
    store(value) {
        this.memory = value;
    }
    recall() {
        return this.memory;
    }
    getHistory() {
        return this.history;
    }
    clearHistory() {
        this.history = [];
    }
}
// Main function
function main() {
    console.log("Calculator Demo (TypeScript)");
    console.log("=".repeat(40));
    // Basic operations
    console.log(`5 + 3 = ${add(5, 3)}`);
    console.log(`10 - 4 = ${subtract(10, 4)}`);
    console.log(`6 * 7 = ${multiply(6, 7)}`);
    console.log(`20 / 4 = ${divide(20, 4)}`);
    // Factorial
    console.log(`5! = ${factorial(5)}`);
    // Fibonacci
    const fib10 = fibonacci(10);
    console.log(`First 10 Fibonacci: ${fib10}`);
    // Using the Calculator class
    const calc = new Calculator();
    calc.calculate("add", 10, 20);
    calc.calculate("multiply", 5, 5);
    calc.calculate("factorial", 6);
    console.log(`\nCalculator history: ${JSON.stringify(calc.getHistory())}`);
    // Store and recall
    calc.store(42);
    console.log(`Memory value: ${calc.recall()}`);
    console.log("\nDone!");
}
main();
//# sourceMappingURL=calculator.js.map