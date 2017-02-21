package main

import (
	"github.com/Sirupsen/logrus"
	"net/http"
	//"time"
)

type WorkRequest struct {
	Request *http.Request
	//Type string (mkdir, create)
	//Name string (dir, file)
}

type Worker struct {
	ID          int
	Work        chan WorkRequest
	WorkerQueue chan chan WorkRequest
	QuitChan    chan bool
}

// A buffered channel that we can send work requests on. 100?
var WorkQueue = make(chan WorkRequest, 10000)

func Collector(req *http.Request) error {

	// Now, we take the http request and make a WorkRequest out of them.
	logrus.Debug("Collector", req)
	work := WorkRequest{Request: req}

	// Push the work onto the queue.
	WorkQueue <- work
	logrus.Debug("Work request queued")

	return nil
}

// NewWorker creates, and returns a new Worker object. Its only argument
// is a channel that the worker can add itself to whenever it is done its
// work.
func NewWorker(id int, workerQueue chan chan WorkRequest) Worker {
	// Create, and return the worker.
	worker := Worker{
		ID:          id,
		Work:        make(chan WorkRequest),
		WorkerQueue: workerQueue,
		QuitChan:    make(chan bool)}

	return worker
}

// This function "starts" the worker by starting a goroutine, that is
// an infinite "for-select" loop.
func (w *Worker) Start() {
	go func() {
		for {
			// Add ourselves into the worker queue.
			w.WorkerQueue <- w.Work

			select {
			case work := <-w.Work:
				// Receive a work request.
				logrus.Debug("Worker received ", w.ID)
				client := &http.Client{}
				resp, err := client.Do(work.Request)
				if err != nil {
					logrus.Error("Error Client.do", err)
				} else {
					defer resp.Body.Close()
				}
				// Handle errors
				/*if resp.StatusCode == 401 {
				      return errors.New(ERROR_NOT_AUTHORIZED)
				  }
				  if err != nil {
				      logrus.Errorf("CreateDir %v", err)
				      return err
				  }*/
				logrus.Debug("Worker Hello ", w.ID)

			case <-w.QuitChan:
				// We have been asked to stop.
				logrus.Debug("Worker stoppping ", w.ID)
				return
			}
		}
	}()
}

// Stop tells the worker to stop listening for work requests.
//
// Note that the worker will only stop *after* it has finished its work.
func (w *Worker) Stop() {
	go func() {
		w.QuitChan <- true
	}()
}

var WorkerQueue chan chan WorkRequest

func StartDispatcher(nworkers int) {
	// First, initialize the channel we are going to but the workers' work channels into.
	WorkerQueue = make(chan chan WorkRequest, nworkers)

	// Now, create all of our workers.
	for i := 0; i < nworkers; i++ {
		logrus.Debug("Starting worker ", i+1)
		worker := NewWorker(i+1, WorkerQueue)
		worker.Start()
	}

	go func() {
		for {
			select {
			case work := <-WorkQueue:
				logrus.Debug("Received work request")
				go func() {
					worker := <-WorkerQueue

					logrus.Debug("Dispatching work request")
					worker <- work
				}()
			}
		}
	}()
}
