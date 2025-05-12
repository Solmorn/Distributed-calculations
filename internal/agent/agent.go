package agent

import (
	"context"
	"log"
	"time"

	"github.com/Solmorn/Distributed-calculations/pkg"
	pb "github.com/Solmorn/Distributed-calculations/proto/generated/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func StartAgent() {
	computingPower := pkg.GetEnvInt("COMPUTING_POWER", 3)
	for i := 0; i < computingPower; i++ {
		go worker(i)
	}
}

func worker(id int) {
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		log.Fatalf("Worker %d failed to connect: %v", id, err)
	}
	defer conn.Close()

	client := pb.NewTaskServiceClient(conn)

	for {
		task, err := client.GetTask(context.Background(), &pb.TaskRequest{})
		if err != nil {
			log.Printf("Worker %d error getting task: %v", id, err)
			time.Sleep(1 * time.Second)
			continue
		}
		if !task.HasTask {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		log.Printf("Worker %d received task: %+v", id, task)

		time.Sleep(time.Duration(task.OperationTime) * time.Millisecond)
		result := compute(task.Arg1, task.Arg2, task.Operation)
		_, err = client.SendTaskResult(context.Background(), &pb.TaskResult{
			Id:     task.Id,
			Result: result,
		})

		if err != nil {
			log.Printf("Worker %d error sending result: %v", id, err)
		} else {
			log.Printf("Worker %d completed task %d with result %f", id, task.Id, result)
		}
	}
}

func compute(arg1, arg2 float64, op string) float64 {
	switch op {
	case "+":
		return arg1 + arg2
	case "-":
		return arg1 - arg2
	case "*":
		return arg1 * arg2
	case "/":
		if arg2 != 0 {
			return arg1 / arg2
		}
	}
	return 0
}
