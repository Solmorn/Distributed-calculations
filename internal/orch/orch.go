package orchestrator

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/Solmorn/Distributed-calculations/internal/auth"
	"github.com/Solmorn/Distributed-calculations/internal/db"
	"github.com/Solmorn/Distributed-calculations/pkg"
	pb "github.com/Solmorn/Distributed-calculations/proto/generated/proto"
	"google.golang.org/grpc"
)

type Expression struct {
	ID     int     `json:"id"`
	Expr   string  `json:"expression"`
	Status string  `json:"status"`
	Result float64 `json:"result"`
}

var (
	taskQueue     = make(chan *pb.Task, 100)
	chTaskResults = make(map[int]chan float64)
	mu            sync.Mutex
)

type TaskServer struct {
	pb.UnimplementedTaskServiceServer
}

func (s *TaskServer) GetTask(ctx context.Context, req *pb.TaskRequest) (*pb.Task, error) {
	select {
	case task := <-taskQueue:
		return task, nil
	default:
		database := db.GetInstance()
		unprocessedTasks, err := database.GetUnprocessedTasks(1)
		if err != nil {
			log.Printf("Error receiving unprocessed tasks: %v", err)
			return &pb.Task{HasTask: false}, nil
		}

		if len(unprocessedTasks) > 0 {
			task := unprocessedTasks[0]
			opTime := getOperationTime(task.Operation)
			return &pb.Task{
				Id:            int32(task.ID),
				Arg1:          task.Arg1,
				Arg2:          task.Arg2,
				Operation:     task.Operation,
				OperationTime: int32(opTime),
				HasTask:       true,
			}, nil
		}

		return &pb.Task{HasTask: false}, nil
	}
}

func (s *TaskServer) SendTaskResult(ctx context.Context, result *pb.TaskResult) (*pb.TaskResponse, error) {
	database := db.GetInstance()
	err := database.UpdateTaskResult(int(result.Id), result.Result)
	if err != nil {
		log.Printf("Error saving the task result: %v", err)
		return &pb.TaskResponse{Success: false}, nil
	}

	mu.Lock()
	ch, exists := chTaskResults[int(result.Id)]
	mu.Unlock()

	if exists {
		ch <- result.Result
	}

	return &pb.TaskResponse{Success: true}, nil
}

func RunOrchestrator() {
	// Запускаем HTTP сервер для API
	go runHTTPServer()

	// Запускаем gRPC сервер
	runGRPCServer()
}

func runHTTPServer() {
	http.HandleFunc("/api/v1/register", handleRegister)
	http.HandleFunc("/api/v1/login", handleLogin)

	http.HandleFunc("/api/v1/calculate", auth.AuthMiddleware(handleCalculate))
	http.HandleFunc("/api/v1/expressions", auth.AuthMiddleware(handleExpressions))
	http.HandleFunc("/api/v1/expressions/", auth.AuthMiddleware(handleExpressionByID))

	log.Println("HTTP server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func runGRPCServer() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterTaskServiceServer(s, &TaskServer{})

	log.Println("gRPC server started on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func handleCalculate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Expr string `json:"expression"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	userID, err := auth.GetUserIDFromContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	database := db.GetInstance()
	lastID, err := database.GetLastExpressionID()
	if err != nil {
		log.Printf("Error getting last ID: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	mu.Lock()
	expressionID := lastID + 1
	err = database.SaveExpression(expressionID, userID, req.Expr, "processing", 0)
	if err != nil {
		log.Printf("Error saving expression: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	chTaskResults[expressionID] = make(chan float64, 1)
	mu.Unlock()

	go parseExpression(expressionID, userID, req.Expr)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int{"id": expressionID})
}

func parseExpression(id int, userID int, expression string) {
	var operations []rune
	var numbers []float64

	// Анализируем вводимые данные:

	for i := 0; i < len(expression); i++ {

		// числа и арифметические знаки записываем в списки
		if expression[i] == '*' || expression[i] == '/' || expression[i] == '+' || expression[i] == '-' {
			operations = append(operations, []rune(expression)[i])

		} else if (expression[i] - '0') <= 9 {
			numbers = append(numbers, float64(expression[i]-'0'))
		}
	}

	// проверяем количество чисел и знаков
	if len(numbers) != len(operations)+1 {
		database := db.GetInstance()
		database.SaveExpression(id, userID, expression, "error", 0)
		return
	}

	// вычисляем преоритетные операции
	for i := 0; i < len(operations); i++ {
		switch operations[i] {
		case '*':
			taskID, res := addTask(id, "*", numbers[i], numbers[i+1])
			if taskID == -1 {
				database := db.GetInstance()
				database.SaveExpression(id, userID, expression, "error", 0)
				return
			}

			numbers = append(append(numbers[:i], res), numbers[i+2:]...)
			operations = append(operations[:i], operations[i+1:]...)
			i--

		case '/':
			if numbers[i+1] == 0 {
				database := db.GetInstance()
				database.SaveExpression(id, userID, expression, "error", 0)
				return
			}

			taskID, res := addTask(id, "/", numbers[i], numbers[i+1])
			if taskID == -1 {
				database := db.GetInstance()
				database.SaveExpression(id, userID, expression, "error", 0)
				return
			}

			numbers = append(append(numbers[:i], res), numbers[i+2:]...)
			operations = append(operations[:i], operations[i+1:]...)
			i--
		}
	}

	// вычисляем менее преоритетные операции
	for i := 0; i < len(operations); i++ {
		switch operations[i] {
		case '+':
			taskID, res := addTask(id, "+", numbers[i], numbers[i+1])
			if taskID == -1 {
				database := db.GetInstance()
				database.SaveExpression(id, userID, expression, "error", 0)
				return
			}
			numbers = append(append(numbers[:i], res), numbers[i+2:]...)
			operations = append(operations[:i], operations[i+1:]...)
			i--

		case '-':
			taskID, res := addTask(id, "-", numbers[i], numbers[i+1])
			if taskID == -1 {
				database := db.GetInstance()
				database.SaveExpression(id, userID, expression, "error", 0)
				return
			}
			numbers = append(append(numbers[:i], res), numbers[i+2:]...)
			operations = append(operations[:i], operations[i+1:]...)
			i--
		}
	}

	database := db.GetInstance()
	database.SaveExpression(id, userID, expression, "completed", numbers[0])
}

func addTask(expressionID int, op string, arg1, arg2 float64) (int, float64) {
	database := db.GetInstance()
	taskID, err := database.SaveTask(expressionID, arg1, arg2, op)
	if err != nil {
		log.Printf("Error saving an task: %v", err)
		return -1, 0
	}

	opTime := getOperationTime(op)
	taskQueue <- &pb.Task{
		Id:            int32(taskID),
		Arg1:          arg1,
		Arg2:          arg2,
		Operation:     op,
		OperationTime: int32(opTime),
		HasTask:       true,
	}

	mu.Lock()
	ch := chTaskResults[expressionID]
	mu.Unlock()

	result := <-ch
	return taskID, result
}

func getOperationTime(op string) int {
	switch op {
	case "+":
		return pkg.GetEnvInt("TIME_ADDITION_MS", 100)
	case "-":
		return pkg.GetEnvInt("TIME_SUBTRACTION_MS", 100)
	case "*":
		return pkg.GetEnvInt("TIME_MULTIPLICATIONS_MS", 200)
	case "/":
		return pkg.GetEnvInt("TIME_DIVISIONS_MS", 300)
	}
	return 100
}

func handleExpressions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	userID, err := auth.GetUserIDFromContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	database := db.GetInstance()
	dbExpressions, err := database.GetAllExpressions(userID)
	if err != nil {
		log.Printf("Error receiving expressions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var expList []Expression
	for _, exp := range dbExpressions {
		expList = append(expList, Expression{
			ID:     exp.ID,
			Expr:   exp.Expression,
			Status: exp.Status,
			Result: exp.Result,
		})
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"expressions": expList})
}

func handleExpressionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	userID, err := auth.GetUserIDFromContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.URL.Path[len("/api/v1/expressions/"):]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	database := db.GetInstance()
	expr, status, result, err := database.GetExpression(id, userID)
	if err != nil {
		http.Error(w, "Expression not found", http.StatusNotFound)
		return
	}

	expression := Expression{
		ID:     id,
		Expr:   expr,
		Status: status,
		Result: result,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"expression": expression})

}

type UserCredentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	var credentials UserCredentials
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Проверка наличия логина и пароля
	if credentials.Login == "" || credentials.Password == "" {
		http.Error(w, "Login and password are required", http.StatusBadRequest)
		return
	}

	// Хеширование пароля
	hashedPassword, err := auth.GeneratePasswordHash(credentials.Password)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Сохранение пользователя в БД
	database := db.GetInstance()
	_, err = database.CreateUser(credentials.Login, hashedPassword)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		http.Error(w, "User already exists or internal error", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	var credentials UserCredentials
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if credentials.Login == "" || credentials.Password == "" {
		http.Error(w, "Login and password are required", http.StatusBadRequest)
		return
	}

	database := db.GetInstance()
	userID, hashedPassword, err := database.GetUserByLogin(credentials.Login)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !auth.CheckPasswordHash(credentials.Password, hashedPassword) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(userID)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}
