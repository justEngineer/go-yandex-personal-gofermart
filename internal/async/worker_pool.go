package async

import (
	"errors"
	"fmt"
	"log"
	"time"

	database "github.com/justEngineer/go-yandex-personal-gofermart/internal/database"
	client "github.com/justEngineer/go-yandex-personal-gofermart/internal/http/client"
)

const (
	countWorker             = 5
	retryTimeCheckNewOrders = 5 * time.Second
	idleTimeout             = time.Minute
	idleTimeoutNoOrders     = time.Second
)

type WorkerPool struct {
	client  *client.AccrualClient
	storage database.Storage
	orderIn chan string
	Err     chan error
}

func NewWorkerPool(client *client.AccrualClient, storage database.Storage) *WorkerPool {
	ordersIn := make(chan string, 10)
	err := make(chan error)
	return &WorkerPool{client: client, storage: storage, orderIn: ordersIn, Err: err}
}

func (p *WorkerPool) Execute() {
	ticker := time.NewTicker(retryTimeCheckNewOrders)
	pauses := make([]chan struct{}, 0)
	for i := 0; i < countWorker; i++ {
		name := i
		pause := make(chan struct{})
		p.worker(name, pause)
		pauses = append(pauses, pause)
	}

	go func() {
		for range ticker.C {
			ordersInProgress, err := p.storage.GetOrderIDs()
			if err != nil {
				log.Println("error start workers of integration")
				break
			}
			if len(ordersInProgress) == 0 {
				log.Println("worker nothing to do, no orders in progress")
				continue
			}
			for _, OrderID := range ordersInProgress {
				ID := OrderID
				p.orderIn <- ID
			}
		}
	}()

	for err := range p.Err {
		log.Printf("error %s", err.Error())
		if errors.Is(err, client.ErrTooManyRequests) {
			go func() {
				for _, pause := range pauses {
					ch := pause
					ch <- struct{}{}
				}
			}()
		}
	}
}

func (p *WorkerPool) worker(nameWorker int, pause chan struct{}) {
	go func() {
		defer close(pause)
		for {
			select {
			case order := <-p.orderIn:
				log.Printf("worker %d, order %s send request to accrual services", nameWorker, order)
				accrual, err := p.client.GetOrderInfo(order)
				if err != nil {
					p.Err <- fmt.Errorf("error worker %d %w", nameWorker, err)
					break
				}
				log.Printf("worker %d, save %v in order", nameWorker, accrual)
				err = p.storage.ApplyAccural(&accrual)
				if err != nil {
					p.Err <- fmt.Errorf("error worker %d %w", nameWorker, err)
				}
			case <-pause:
				log.Printf("worker %d do pause", nameWorker)
				time.Sleep(idleTimeout)
			}
		}
	}()
}
