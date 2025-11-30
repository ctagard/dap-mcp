import { useState } from 'react'

// Calculator functions - good debugging targets
function add(a, b) {
  const result = a + b  // Line 5: Set breakpoint here
  return result
}

function subtract(a, b) {
  const result = a - b  // Line 10: Set breakpoint here
  return result
}

function multiply(a, b) {
  const result = a * b  // Line 15: Set breakpoint here
  return result
}

function divide(a, b) {
  if (b === 0) {
    return 'Error: Division by zero'  // Line 21: Set breakpoint here
  }
  const result = a / b  // Line 23: Set breakpoint here
  return result
}

function App() {
  const [num1, setNum1] = useState(10)
  const [num2, setNum2] = useState(5)
  const [result, setResult] = useState(null)
  const [operation, setOperation] = useState(null)

  const handleCalculate = (op) => {
    let calcResult
    setOperation(op)

    switch (op) {
      case 'add':
        calcResult = add(num1, num2)  // Line 39: Set breakpoint here
        break
      case 'subtract':
        calcResult = subtract(num1, num2)  // Line 42: Set breakpoint here
        break
      case 'multiply':
        calcResult = multiply(num1, num2)  // Line 45: Set breakpoint here
        break
      case 'divide':
        calcResult = divide(num1, num2)  // Line 48: Set breakpoint here
        break
      default:
        calcResult = null
    }

    setResult(calcResult)  // Line 54: Set breakpoint here
    console.log(`Calculated: ${num1} ${op} ${num2} = ${calcResult}`)
  }

  return (
    <div style={{ padding: '20px', fontFamily: 'sans-serif' }}>
      <h1>React Calculator - Debug Test</h1>

      <div style={{ marginBottom: '20px' }}>
        <label>
          Number 1:
          <input
            type="number"
            value={num1}
            onChange={(e) => setNum1(Number(e.target.value))}
            style={{ marginLeft: '10px', width: '80px' }}
          />
        </label>
      </div>

      <div style={{ marginBottom: '20px' }}>
        <label>
          Number 2:
          <input
            type="number"
            value={num2}
            onChange={(e) => setNum2(Number(e.target.value))}
            style={{ marginLeft: '10px', width: '80px' }}
          />
        </label>
      </div>

      <div style={{ marginBottom: '20px' }}>
        <button onClick={() => handleCalculate('add')} style={{ marginRight: '10px' }}>
          Add
        </button>
        <button onClick={() => handleCalculate('subtract')} style={{ marginRight: '10px' }}>
          Subtract
        </button>
        <button onClick={() => handleCalculate('multiply')} style={{ marginRight: '10px' }}>
          Multiply
        </button>
        <button onClick={() => handleCalculate('divide')}>
          Divide
        </button>
      </div>

      {result !== null && (
        <div style={{ fontSize: '24px', fontWeight: 'bold' }}>
          Result: {num1} {operation} {num2} = {result}
        </div>
      )}

      <div style={{ marginTop: '40px', color: '#666', fontSize: '14px' }}>
        <h3>Debug Test Instructions:</h3>
        <ol>
          <li>Set a breakpoint on line 39 (add operation)</li>
          <li>Click the "Add" button</li>
          <li>The debugger should pause at the breakpoint</li>
          <li>Inspect variables: num1, num2, op</li>
        </ol>
      </div>
    </div>
  )
}

export default App
