/*
 * @Author: tuxer
 * @Date: 2020-06-18 14:05:19
 * @Last Modified by: tuxer
 * @Last Modified time: 2020-06-19 14:50:43
 */

package service

import (
	"errors"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"syscall"
)

var (
	exitSignal chan os.Signal
	services   = []*Service{}

	doneSignal chan bool
)

//WritePID ...
func WritePID(filename string) error {
	data, e := ioutil.ReadFile(filename)
	if e == nil {
		pid, e := strconv.Atoi(string(data))
		if e != nil {
			os.Remove(filename)
		} else {
			proc, _ := os.FindProcess(pid)
			if e := proc.Signal(syscall.Signal(0)); e != nil {
				os.Remove(filename)
			} else {
				return errors.New(`service already running`)
			}
		}
	}
	if e := ioutil.WriteFile(filename, []byte(strconv.Itoa(os.Getpid())), 0644); e != nil {
		return e
	}
	return nil
}

type worker struct {
	id   int
	done chan int
}

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

//Start ...
//  func (s *Service) Start() error {
// 	 if s.done != nil {
// 		 return errors.New(`service already running`)
// 	 }
// 	 s.done = make(chan bool)
// 	 s.sign = make(chan os.Signal, 1)
// 	 signal.Notify(s.sign, syscall.SIGINT, syscall.SIGTERM)

// 	 go func() {
// 		 <-s.sign
// 		 println(`service stopping`)
// 		 close(s.queue)
// 	 }()

// 	 go func() {
// 		 for s.queue != nil {
// 			 if w, ok := <-s.queue; ok {
// 				 go func(w *worker) {
// 					 defer func() {
// 						 if r := recover(); r != nil {
// 							 println(r)
// 						 }
// 					 }()
// 					 s.wg.Add(1)
// 					 s.fn(w.done)
// 					 if s.queue != nil {
// 						 s.queue <- w
// 					 }
// 					 s.wg.Done()
// 				 }(w)
// 			 } else {
// 				 s.queue = nil
// 			 }
// 		 }
// 		 for i := 0; i < len(s.workers); i++ {
// 			 w := s.workers[i]
// 			 go func(w *worker) {
// 				 w.done <- w.id
// 			 }(w)
// 		 }

// 		 s.wg.Wait()
// 		 s.done <- true
// 	 }()

// 	 return nil
//  }

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
							println(r)
						}
					}()
					s.wg.Add(1)
					s.fn(w.done)
					if s.queue != nil {
						s.queue <- w
					}
					s.wg.Done()
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

//Start ...
func Start() error {
	if exitSignal == nil {
		return errors.New(`services already running`)
	}
	for _, s := range services {
		s.start()
	}
	exitSignal = make(chan os.Signal, 1)
	doneSignal = make(chan bool, 1)
	go func() {
		<-exitSignal
		Stop()
	}()
	return nil
}

//Stop ...
func Stop() {
	println(`Service stopping`)
	wg := sync.WaitGroup{}
	for _, s := range services {
		go func(s *Service) {
			wg.Add(1)
			defer wg.Done()
			close(s.queue)
			<-s.doneCh
		}(s)
	}
	wg.Wait()
	if len(doneSignal) == 0 {
		doneSignal <- true
	}
}

//Wait ...
func Wait() {
	if doneSignal == nil {
		return
	}
	<-doneSignal
}

//New ...
func New(fn func(<-chan int), max int) *Service {
	s := &Service{fn: fn, queue: make(chan *worker, max)}
	for len(s.queue) < cap(s.queue) {
		w := &worker{id: len(s.queue) + 1, done: make(chan int, 1)}
		s.workers = append(s.workers, w)
		s.queue <- w
	}
	return s
}
