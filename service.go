package command

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

//Service ...
type Service struct {
	fn      func(<-chan int)
	queue   chan *worker
	workers []*worker
	doneCh  chan bool
	running bool
}

//AvailableThread ...
func (s *Service) AvailableThread() int {
	return len(s.queue)
}

//MaxThread ...
func (s *Service) MaxThread() int {
	return cap(s.queue)
}

func (s *Service) start() {
	if s.doneCh != nil {
		return
	}
	s.doneCh = make(chan bool)
	s.running = true

	go s.run()
}

func (s *Service) run() {
	wg := sync.WaitGroup{}

	for s.running {
		if w, ok := <-s.queue; ok {
			go func(w *worker) {
				defer func() {
					if r := recover(); r != nil {
						errorLog(r)
					}
				}()
				wg.Add(1)
				defer wg.Done()
				s.fn(w.done)
				if s.running {
					s.queue <- w
				}
			}(w)
		} else {
			s.running = false
		}
	}
	wg.Wait()
	s.running = false
	s.doneCh <- true
}

func (s *Service) stop() {
	if s.running == false {
		return
	}
	s.running = false
	if s.queue != nil {
		close(s.queue)
		s.queue = nil
	}
	for i := 0; i < len(s.workers); i++ {
		w := s.workers[i]
		w.done <- w.id
	}
}

//StartService ...
func StartService(args ...string) error {
	cmd := exec.Command(`./`+Filename(), args...)
	if e := cmd.Start(); e != nil {
		return e
	}
	os.MkdirAll(filepath.Dir(pidfile), 0755)
	if e := ioutil.WriteFile(pidfile+`.pid`, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); e != nil {
		return e
	}
	return nil
}

//StopService ...
func StopService() error {
	data, e := ioutil.ReadFile(pidfile + `.pid`)
	if e != nil {
		return e
	}
	pid, e := strconv.Atoi(strings.TrimSpace(string(data)))
	if e != nil {
		return e
	}
	proc, e := os.FindProcess(pid)
	if e != nil {
		return e
	}
	return proc.Signal(os.Interrupt)
}
