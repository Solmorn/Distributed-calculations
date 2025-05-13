package orch

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Solmorn/Distributed-calculations/internal/auth"
	"github.com/Solmorn/Distributed-calculations/internal/db"
	pb "github.com/Solmorn/Distributed-calculations/proto/generated/proto"
)

func TestMain(m *testing.M) {
	os.Setenv("DB_PATH", ":memory:")

	taskQueue = make(chan *pb.Task, 100)
	chTaskResults = make(map[int]chan float64)

	code := m.Run()

	os.Exit(code)
}

func TestGetTask(t *testing.T) {
	database := db.GetInstance()

	userID, err := database.CreateUser("testtask", "password")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	lastID, err := database.GetLastExpressionID()
	if err != nil {
		t.Fatalf("Failed to get last expression ID: %v", err)
	}

	expressionID := lastID + 1
	err = database.SaveExpression(expressionID, userID, "5+5", "processing", 0)
	if err != nil {
		t.Fatalf("Failed to save expression: %v", err)
	}

	taskID, err := database.SaveTask(expressionID, 5.0, 5.0, "+")
	if err != nil {
		t.Fatalf("Failed to create test task: %v", err)
	}

	server := &TaskServer{}
	task, err := server.GetTask(context.Background(), &pb.TaskRequest{})
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if !task.HasTask {
		t.Error("Expected task, got none")
	}

	if int(task.Id) != taskID {
		t.Errorf("Task ID mismatch: expected %d, got %d", taskID, task.Id)
	}

	if task.Arg1 != 5.0 || task.Arg2 != 5.0 || task.Operation != "+" {
		t.Errorf("Task data mismatch: expected (5.0, 5.0, +), got (%f, %f, %s)",
			task.Arg1, task.Arg2, task.Operation)
	}
}

func TestSendTaskResult(t *testing.T) {
	database := db.GetInstance()

	userID, err := database.CreateUser("testresult", "password")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	lastID, err := database.GetLastExpressionID()
	if err != nil {
		t.Fatalf("Failed to get last expression ID: %v", err)
	}

	expressionID := lastID + 1
	err = database.SaveExpression(expressionID, userID, "10+5", "processing", 0)
	if err != nil {
		t.Fatalf("Failed to save expression: %v", err)
	}

	taskID, err := database.SaveTask(expressionID, 10.0, 5.0, "+")
	if err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	mu.Lock()
	chTaskResults[expressionID] = make(chan float64, 1)
	mu.Unlock()

	server := &TaskServer{}
	result := &pb.TaskResult{
		Id:     int32(taskID),
		Result: 15.0,
	}

	response, err := server.SendTaskResult(context.Background(), result)
	if err != nil {
		t.Fatalf("SendTaskResult failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected success response, got failure")
	}

	resultValue, processed, err := database.GetTaskResult(taskID)
	if err != nil {
		t.Fatalf("Failed to get task result: %v", err)
	}

	if !processed {
		t.Error("Task should be marked as processed")
	}

	if resultValue != 15.0 {
		t.Errorf("Result mismatch: expected 15.0, got %f", resultValue)
	}
}

func TestHandleCalculate(t *testing.T) {
	reqBody := []byte(`{"expression": "3+4"}`)
	req := httptest.NewRequest("POST", "/api/v1/calculate", bytes.NewBuffer(reqBody))

	database := db.GetInstance()
	userID, err := database.CreateUser("calcuser", "password")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	token, err := auth.GenerateToken(userID)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()

	ctx := req.Context()
	ctx = context.WithValue(ctx, auth.GetUserIDContextKey(), userID)
	req = req.WithContext(ctx)

	handler := http.HandlerFunc(handleCalculate)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Handler returned wrong status code: got %v want %v, body: %s",
			status, http.StatusCreated, rr.Body.String())
	}

	var response map[string]int
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, exists := response["id"]; !exists {
		t.Error("Response should contain expression ID")
	}

	expr, status, _, err := database.GetExpression(response["id"], userID)
	if err != nil {
		t.Fatalf("Failed to get expression: %v", err)
	}

	if expr != "3+4" {
		t.Errorf("Expression mismatch: expected 3+4, got %s", expr)
	}

	if status != "processing" && status != "completed" {
		t.Errorf("Status should be processing or completed, got %s", status)
	}
}

func TestHandleExpressions(t *testing.T) {
	database := db.GetInstance()

	userID, err := database.CreateUser("expruser", "password")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	lastID, err := database.GetLastExpressionID()
	if err != nil {
		t.Fatalf("Failed to get last expression ID: %v", err)
	}

	expressionID := lastID + 1
	err = database.SaveExpression(expressionID, userID, "7*8", "completed", 56.0)
	if err != nil {
		t.Fatalf("Failed to save expression: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/expressions", nil)

	token, err := auth.GenerateToken(userID)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	ctx := req.Context()
	ctx = context.WithValue(ctx, auth.GetUserIDContextKey(), userID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(handleExpressions)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v, body: %s",
			status, http.StatusOK, rr.Body.String())
	}

	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	expressions, exists := response["expressions"].([]interface{})
	if !exists {
		t.Fatal("Response should contain expressions array")
	}

	if len(expressions) < 1 {
		t.Error("Expected at least one expression in response")
	}

	expr := expressions[0].(map[string]interface{})
	if expr["expression"] != "7*8" {
		t.Errorf("Expression mismatch: expected 7*8, got %s", expr["expression"])
	}

	if expr["status"] != "completed" {
		t.Errorf("Status mismatch: expected completed, got %s", expr["status"])
	}

	if expr["result"].(float64) != 56.0 {
		t.Errorf("Result mismatch: expected 56.0, got %f", expr["result"])
	}
}

func TestParseExpression(t *testing.T) {
	database := db.GetInstance()
	userID, err := database.CreateUser("parseuser", "password")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	expressionID := 1000
	expression := "2+3"

	mu.Lock()
	chTaskResults[expressionID] = make(chan float64, 1)

	go func() {
		chTaskResults[expressionID] <- 5.0
	}()
	mu.Unlock()

	parseExpression(expressionID, userID, expression)

	expr, status, result, err := database.GetExpression(expressionID, userID)
	if err != nil {
		t.Fatalf("Failed to get expression: %v", err)
	}

	if expr != "2+3" {
		t.Errorf("Expression mismatch: expected 2+3, got %s", expr)
	}

	if status != "completed" {
		t.Errorf("Status mismatch: expected completed, got %s", status)
	}

	if result != 5.0 {
		t.Errorf("Result mismatch: expected 5.0, got %f", result)
	}
}
