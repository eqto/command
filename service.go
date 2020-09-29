package command

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

//Service ...
type Service struct {
	fn      func(<-chan int)
	queue   chan *worker
	workers []*worker
	wg      sync.WaitGroup
	doneCh  chan bool
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

	go func() {
		for s.queue != nil {
			if w, ok := <-s.queue; ok {
				go func(w *worker) {
					defer func() {
						if r := recover(); r != nil {
							errorLog(r)
						}
					}()
					s.wg.Add(1)
					defer s.wg.Done()
					s.fn(w.done)
					if s.queue != nil {
						s.queue <- w
					}
				}(w)
			} else {
				s.queue = nil
			}
		}
		for i := 0; i < len(s.workers); i++ {
			w := s.workers[i]
			go func(w *worker) {
				w.done <- w.id
			}(w)
		}

		s.wg.Wait()
		s.doneCh <- true
	}()
}

//StartService ...
func StartService(args ...string) error {
	cmd := exec.Command(`./`+Filename(), args...)
	if e := cmd.Start(); e != nil {
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
