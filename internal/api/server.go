package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/google/uuid"
	"godot-builder/internal/jobs"
)

type server struct {
	jobStore   *jobs.Store
	buildQueue chan *jobs.Job
}

func Run() error {
	dbPath := os.Getenv("JOBS_DB_PATH")
	if dbPath == "" {
		dbPath = "tmp/jobs.sqlite"
	}

	jobStore, err := jobs.Open(dbPath)
	if err != nil {
		return err
	}
	defer jobStore.Close()

	s := &server{
		jobStore:   jobStore,
		buildQueue: make(chan *jobs.Job, 100),
	}
	go s.worker()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /build", s.handleStartBuild)
	mux.HandleFunc("GET /build/{id}", s.handleGetJob)

	addr := ":3000"
	log.Printf("listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *server) handleStartBuild(w http.ResponseWriter, r *http.Request) {
	job, err := s.startBuild()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(job)
}

func (s *server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.jobStore.Get(r.PathValue("id"))
	if errors.Is(err, jobs.ErrNotFound) {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (s *server) worker() {
	for job := range s.buildQueue {
		s.runBuild(job)
	}
}

func (s *server) runBuild(job *jobs.Job) {
	job.Status = "running"
	started := time.Now()
	job.Started = &started
	if err := s.jobStore.Update(job); err != nil {
		log.Printf("could not update job %s: %v", job.ID, err)
	}

	logFile, err := os.Create(job.LogPath)
	if err != nil {
		job.Status = "failed"
		job.Err = err.Error()
		ended := time.Now()
		job.Ended = &ended
		if updateErr := s.jobStore.Update(job); updateErr != nil {
			log.Printf("could not update job %s: %v", job.ID, updateErr)
		}
		return
	}
	defer logFile.Close()

	cmd := exec.Command("bash", "scripts/build.sh")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	err = cmd.Run()

	ended := time.Now()
	job.Ended = &ended
	if err == nil {
		job.Status = "done"
	} else {
		job.Status = "failed"
		job.Err = err.Error()
	}
	if updateErr := s.jobStore.Update(job); updateErr != nil {
		log.Printf("could not update job %s: %v", job.ID, updateErr)
	}
}

func (s *server) startBuild() (*jobs.Job, error) {
	id := uuid.NewString()
	job := &jobs.Job{
		ID:      id,
		Status:  "queued",
		LogPath: fmt.Sprintf("tmp/build-%s.log", id),
	}

	if err := s.jobStore.Create(job); err != nil {
		return nil, err
	}

	s.buildQueue <- job
	return job, nil
}
