package db

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Setenv("DB_PATH", ":memory:")

	code := m.Run()

	database := GetInstance()
	database.Close()

	os.Exit(code)
}

func TestCreateAndGetUser(t *testing.T) {
	database := GetInstance()

	testLogin := "testuser"
	testPassword := "hashedpassword123"

	userID, err := database.CreateUser(testLogin, testPassword)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if userID <= 0 {
		t.Errorf("Expected positive user ID, got %d", userID)
	}

	retrievedID, retrievedPassword, err := database.GetUserByLogin(testLogin)
	if err != nil {
		t.Fatalf("Failed to get user by login: %v", err)
	}

	if retrievedID != userID {
		t.Errorf("User ID mismatch: expected %d, got %d", userID, retrievedID)
	}

	if retrievedPassword != testPassword {
		t.Errorf("Password mismatch: expected %s, got %s", testPassword, retrievedPassword)
	}

	_, err = database.CreateUser(testLogin, "anotherpassword")
	if err == nil {
		t.Error("Expected error when creating duplicate user, got nil")
	}
}

func TestExpressionOperations(t *testing.T) {
	database := GetInstance()

	userID, err := database.CreateUser("expruser", "password")
	if err != nil {
		t.Fatalf("Failed to create user for expression test: %v", err)
	}

	lastID, err := database.GetLastExpressionID()
	if err != nil {
		t.Fatalf("Failed to get last expression ID: %v", err)
	}

	expressionID := lastID + 1
	testExpr := "2+2"
	testStatus := "processing"
	var testResult float64 = 0

	err = database.SaveExpression(expressionID, userID, testExpr, testStatus, testResult)
	if err != nil {
		t.Fatalf("Failed to save expression: %v", err)
	}

	expr, status, result, err := database.GetExpression(expressionID, userID)
	if err != nil {
		t.Fatalf("Failed to get expression: %v", err)
	}

	if expr != testExpr {
		t.Errorf("Expression mismatch: expected %s, got %s", testExpr, expr)
	}

	if status != testStatus {
		t.Errorf("Status mismatch: expected %s, got %s", testStatus, status)
	}

	if result != testResult {
		t.Errorf("Result mismatch: expected %f, got %f", testResult, result)
	}

	updatedStatus := "completed"
	var updatedResult float64 = 4

	err = database.SaveExpression(expressionID, userID, testExpr, updatedStatus, updatedResult)
	if err != nil {
		t.Fatalf("Failed to update expression: %v", err)
	}

	_, status, result, err = database.GetExpression(expressionID, userID)
	if err != nil {
		t.Fatalf("Failed to get updated expression: %v", err)
	}

	if status != updatedStatus {
		t.Errorf("Updated status mismatch: expected %s, got %s", updatedStatus, status)
	}

	if result != updatedResult {
		t.Errorf("Updated result mismatch: expected %f, got %f", updatedResult, result)
	}

	allExpressions, err := database.GetAllExpressions(userID)
	if err != nil {
		t.Fatalf("Failed to get all expressions: %v", err)
	}

	if len(allExpressions) != 1 {
		t.Errorf("Expected 1 expression, got %d", len(allExpressions))
	}

	if allExpressions[0].ID != expressionID {
		t.Errorf("Expression ID mismatch: expected %d, got %d", expressionID, allExpressions[0].ID)
	}
}

func TestTaskOperations(t *testing.T) {
	database := GetInstance()

	userID, _ := database.CreateUser("taskuser", "password")
	lastID, _ := database.GetLastExpressionID()
	expressionID := lastID + 1
	database.SaveExpression(expressionID, userID, "3*4", "processing", 0)

	arg1 := 3.0
	arg2 := 4.0
	operation := "*"

	taskID, err := database.SaveTask(expressionID, arg1, arg2, operation)
	if err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	if taskID <= 0 {
		t.Errorf("Expected positive task ID, got %d", taskID)
	}

	tasks, err := database.GetUnprocessedTasks(10)
	if err != nil {
		t.Fatalf("Failed to get unprocessed tasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("Expected 1 unprocessed task, got %d", len(tasks))
	}

	if tasks[0].ID != taskID {
		t.Errorf("Task ID mismatch: expected %d, got %d", taskID, tasks[0].ID)
	}

	if tasks[0].Arg1 != arg1 {
		t.Errorf("Arg1 mismatch: expected %f, got %f", arg1, tasks[0].Arg1)
	}

	if tasks[0].Arg2 != arg2 {
		t.Errorf("Arg2 mismatch: expected %f, got %f", arg2, tasks[0].Arg2)
	}

	if tasks[0].Operation != operation {
		t.Errorf("Operation mismatch: expected %s, got %s", operation, tasks[0].Operation)
	}

	result := 12.0
	err = database.UpdateTaskResult(taskID, result)
	if err != nil {
		t.Fatalf("Failed to update task result: %v", err)
	}

	retrievedResult, processed, err := database.GetTaskResult(taskID)
	if err != nil {
		t.Fatalf("Failed to get task result: %v", err)
	}

	if !processed {
		t.Error("Task should be marked as processed")
	}

	if retrievedResult != result {
		t.Errorf("Result mismatch: expected %f, got %f", result, retrievedResult)
	}

	tasks, _ = database.GetUnprocessedTasks(10)
	for _, task := range tasks {
		if task.ID == taskID {
			t.Error("Processed task should not be in unprocessed tasks list")
		}
	}
}
