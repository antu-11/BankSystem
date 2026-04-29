// Package email provides an asynchronous email worker pool that processes
// jobs from a buffered channel without blocking HTTP request handlers.
package email

import (
	"fmt"
	"log"
	"net/smtp"
	"sync"
)

// JobType identifies which email template to use.
type JobType string

const (
	JobWelcome            JobType = "welcome"
	JobTransactionSuccess JobType = "transaction_success"
	JobTransactionFailed  JobType = "transaction_failed"
)

// Job is a unit of work submitted to the email worker pool.
type Job struct {
	Type      JobType
	To        string            // recipient email
	Username  string            // display name
	ExtraData map[string]string // template-specific data
}

// SMTPConfig holds the credentials and host for outgoing mail.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// Worker is an asynchronous email processor backed by a buffered channel.
type Worker struct {
	jobs chan Job
	smtp SMTPConfig
	wg   sync.WaitGroup
	quit chan struct{}
}

// NewWorker creates a Worker with the given SMTP configuration and queue capacity.
func NewWorker(cfg SMTPConfig, poolSize, queueSize int) *Worker {
	return &Worker{
		jobs: make(chan Job, queueSize),
		smtp: cfg,
		quit: make(chan struct{}),
	}
}

// Start launches poolSize worker goroutines. Call once at server startup.
func (w *Worker) Start(poolSize int) {
	for i := 0; i < poolSize; i++ {
		w.wg.Add(1)
		go w.process(i)
	}
	log.Printf("[INFO] email worker pool started with %d workers", poolSize)
}

// Enqueue submits an email job. Non-blocking: if the queue is full the job
// is dropped and a warning is logged.
func (w *Worker) Enqueue(job Job) {
	select {
	case w.jobs <- job:
		log.Printf("[EMAIL] queued %s email for %s", job.Type, job.To)
	default:
		log.Printf("[WARN] email queue full — dropped %s for %s", job.Type, job.To)
	}
}

// Shutdown signals all workers to drain the queue and exit.
func (w *Worker) Shutdown() {
	close(w.quit)
	close(w.jobs)
	w.wg.Wait()
	log.Println("[INFO] email worker pool stopped")
}

func (w *Worker) process(id int) {
	defer w.wg.Done()

	for {
		select {
		case job, ok := <-w.jobs:
			if !ok {
				return
			}
			if err := w.send(job); err != nil {
				log.Printf("[ERROR] worker %d: failed to send %s to %s: %v", id, job.Type, job.To, err)
			} else {
				log.Printf("[EMAIL] worker %d: sent %s to %s", id, job.Type, job.To)
			}
		case <-w.quit:
			return
		}
	}
}

func (w *Worker) send(job Job) error {
	subject, body := RenderTemplate(job)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		w.smtp.From, job.To, subject, body,
	)

	auth := smtp.PlainAuth("", w.smtp.Username, w.smtp.Password, w.smtp.Host)
	addr := fmt.Sprintf("%s:%s", w.smtp.Host, w.smtp.Port)

	return smtp.SendMail(addr, auth, w.smtp.From, []string{job.To}, []byte(msg))
}
