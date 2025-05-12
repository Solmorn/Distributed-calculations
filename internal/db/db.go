package db

import (
	"database/sql"
	"log"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
	mu sync.Mutex
}

var (
	instance *Database
	once     sync.Once
)

func (d *Database) initDB() {
	_, err := d.db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		login TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)
	`)
	if err != nil {
		log.Fatalf("Error creating users table: %v", err)
	}

	_, err = d.db.Exec(`
	CREATE TABLE IF NOT EXISTS expressions (
		id INTEGER PRIMARY KEY,
		user_id INTEGER NOT NULL,
		expression TEXT NOT NULL,
		status TEXT NOT NULL,
		result REAL
	)
	`)
	if err != nil {
		log.Fatalf("Error make expressions db: %v", err)
	}

	_, err = d.db.Exec(`
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		expression_id INTEGER NOT NULL,
		arg1 REAL NOT NULL,
		arg2 REAL NOT NULL,
		operation TEXT NOT NULL,
		processed BOOLEAN DEFAULT FALSE,
		result REAL,
		FOREIGN KEY (expression_id) REFERENCES expressions(id)
	)
	`)
	if err != nil {
		log.Fatalf("Error make tasks db: %v", err)
	}
}

func (d *Database) CreateUser(login, hashedPassword string) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	res, err := d.db.Exec(
		"INSERT INTO users (login, password) VALUES (?, ?)",
		login, hashedPassword,
	)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func (d *Database) GetUserByLogin(login string) (int, string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var id int
	var password string
	err := d.db.QueryRow(
		"SELECT id, password FROM users WHERE login = ?",
		login,
	).Scan(&id, &password)
	if err != nil {
		return 0, "", err
	}
	return id, password, nil
}

func GetInstance() *Database {
	once.Do(func() {
		db, err := sql.Open("sqlite3", "./calculator.db")
		if err != nil {
			log.Fatal("ERROR open db")
		}

		instance = &Database{db: db}
		instance.initDB()
	})
	return instance
}

func (d *Database) GetLastExpressionID() (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var id int
	err := d.db.QueryRow("SELECT COALESCE(MAX(id), 0) FROM expressions").Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (d *Database) SaveExpression(id int, userID int, expression string, status string, result float64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM expressions WHERE id = ?", id).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		_, err = d.db.Exec(
			"UPDATE expressions SET expression = ?, status = ?, result = ? WHERE id = ?",
			expression, status, result, id,
		)
	} else {
		_, err = d.db.Exec(
			"INSERT INTO expressions (id, user_id, expression, status, result) VALUES (?, ?, ?, ?, ?)",
			id, userID, expression, status, result,
		)
	}
	return err
}

func (d *Database) GetExpression(id int, userID int) (string, string, float64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var expr string
	var status string
	var result float64
	err := d.db.QueryRow(
		"SELECT expression, status, result FROM expressions WHERE id = ? AND user_id = ?",
		id, userID,
	).Scan(&expr, &status, &result)
	if err != nil {
		return "", "", 0, err
	}
	return expr, status, result, nil
}

func (d *Database) GetAllExpressions(userID int) ([]struct {
	ID         int
	Expression string
	Status     string
	Result     float64
}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query("SELECT id, expression, status, result FROM expressions WHERE user_id = ? ORDER BY id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expressions []struct {
		ID         int
		Expression string
		Status     string
		Result     float64
	}

	for rows.Next() {
		var exp struct {
			ID         int
			Expression string
			Status     string
			Result     float64
		}
		if err := rows.Scan(&exp.ID, &exp.Expression, &exp.Status, &exp.Result); err != nil {
			return nil, err
		}
		expressions = append(expressions, exp)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return expressions, nil
}

func (d *Database) SaveTask(expressionID int, arg1, arg2 float64, operation string) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	res, err := d.db.Exec(
		"INSERT INTO tasks (expression_id, arg1, arg2, operation) VALUES (?, ?, ?, ?)",
		expressionID, arg1, arg2, operation,
	)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func (d *Database) UpdateTaskResult(taskID int, result float64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		"UPDATE tasks SET processed = TRUE, result = ? WHERE id = ?",
		result, taskID,
	)
	return err
}

func (d *Database) GetUnprocessedTasks(limit int) ([]struct {
	ID           int
	ExpressionID int
	Arg1         float64
	Arg2         float64
	Operation    string
}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query(
		"SELECT id, expression_id, arg1, arg2, operation FROM tasks WHERE processed = FALSE LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []struct {
		ID           int
		ExpressionID int
		Arg1         float64
		Arg2         float64
		Operation    string
	}

	for rows.Next() {
		var task struct {
			ID           int
			ExpressionID int
			Arg1         float64
			Arg2         float64
			Operation    string
		}
		if err := rows.Scan(&task.ID, &task.ExpressionID, &task.Arg1, &task.Arg2, &task.Operation); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (d *Database) GetTaskResult(taskID int) (float64, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var result float64
	var processed bool
	err := d.db.QueryRow(
		"SELECT result, processed FROM tasks WHERE id = ?",
		taskID,
	).Scan(&result, &processed)
	if err != nil {
		return 0, false, err
	}
	return result, processed, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}
