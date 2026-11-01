package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TODO: Setup SQLite with Go
func main() {
	// Single background worker
	go worker()

	mux := http.NewServeMux()

	mux.HandleFunc("POST /build", handleStartBuild)
	mux.HandleFunc("GET /build/{id}", handleGetJob)

	addr := ":3000"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

/* === Handlers ===  */

func handleStartBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	job, err := startBuild()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(job)
}

func handleGetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")

	job, ok := getJob(id)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

/* === Builder Logic === */

type Job struct {
	ID      string
	Status  string // "running", "done", "failed"
	Err     error
	Started time.Time
	Ended   time.Time
	LogPath string
}

var (
	mu         sync.Mutex
	jobs       = map[string]*Job{}
	buildQueue = make(chan *Job, 100)
)

func init() {
	go worker()
}

func worker() {
	for job := range buildQueue {
		runBuild(job)
	}
}

func runBuild(job *Job) {
	mu.Lock()
	job.Status = "running"
	job.Started = time.Now()
	mu.Unlock()

	logFile, err := os.Create(job.LogPath)
	if err != nil {
		mu.Lock()
		job.Status = "failed"
		job.Err = err
		job.Ended = time.Now()
		mu.Unlock()
		return
	}
	defer logFile.Close()

	cmd := exec.Command("bash", "scripts/build.sh")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	err = cmd.Run()

	mu.Lock()
	job.Ended = time.Now()
	job.Err = err
	if err == nil {
		job.Status = "done"
	} else {
		job.Status = "failed"
	}
	mu.Unlock()
}

func getJob(id string) (*Job, bool) {
	mu.Lock()
	defer mu.Unlock()
	job, ok := jobs[id]
	return job, ok
}

// TODO test
func startBuild() (*Job, error) {
	id := uuid.NewString()
	job := &Job{
		ID:      id,
		Status:  "queued",
		LogPath: fmt.Sprintf("tmp/build%s.log", id),
	}

	mu.Lock()
	jobs[id] = job
	mu.Unlock()

	buildQueue <- job
	return job, nil
}
