package command

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var (
	exitSignal chan os.Signal
	services   = []*Service{}

	doneSignal chan bool

	debugLog = log.Println
	infoLog  = log.Println
	errorLog = log.Println

	pidfile     = ``
	hadWritePID bool
)

//Log ...
func Log(d, i, e func(...interface{})) {
	debugLog = d
	infoLog = i
	errorLog = e
}

func writePID() error {
	if !hadWritePID {
		os.MkdirAll(filepath.Dir(pidfile), 0755)
		if e := ioutil.WriteFile(pidfile+`.pid`, []byte(strconv.Itoa(os.Getpid())), 0644); e != nil {
			return e
		}
	}
	return nil
}

//Filename get application name
func Filename() string {
	_, file := filepath.Split(os.Args[0])
	ext := filepath.Ext(file)
	return file[:len(file)-len(ext)]
}

//ProcessExist return error when not exist (linux only)
func ProcessExist() error {
	data, e := ioutil.ReadFile(pidfile + `.pid`)
	if e != nil {
		return e
	}
	_, e = strconv.Atoi(string(data))
	if e != nil {
		return errors.New(`pid file not found`)
	}
	cmd := exec.Command(`ps`, `-p`, string(data), `-o`, `comm=`)
	out, e := cmd.Output()

	if e != nil {
		return errors.New(`unable to check process id: ` + string(data))
	}

	if strings.TrimSpace(string(out)) != Filename() {
		return errors.New(`pid not match`)
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

//SetPidFile ...
func SetPidFile(name string) {
	pidfile = name
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

//Start ...
func Start() error {
	writePID()
	if exitSignal != nil {
		return errors.New(`services already running`)
	}
	for _, s := range services {
		s.start()
	}
	exitSignal = make(chan os.Signal, 1)
	doneSignal = make(chan bool, 1)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGQUIT)
	go func() {
		<-exitSignal
		Stop()
	}()
	return nil
}

//Stop ...
func Stop() {
	wg := sync.WaitGroup{}
	wg.Add(len(services))
	for _, s := range services {
		go func(s *Service) {
			defer wg.Done()
			if s.queue != nil {
				close(s.queue)
			}
			<-s.doneCh
		}(s)
	}
	wg.Wait()
	if len(doneSignal) == 0 {
		doneSignal <- true
	}
	os.Remove(pidfile + `.pid`)
}

//Wait ...
func Wait() {
	if doneSignal == nil {
		return
	}
	<-doneSignal
}

//New ...
//!deprecated use Add() instead
func New(fn func(<-chan int), max int) *Service {
	return Add(fn, max)
}

//Add ...
func Add(fn func(<-chan int), max int) *Service {
	s := &Service{fn: fn, queue: make(chan *worker, max)}
	for len(s.queue) < cap(s.queue) {
		w := &worker{id: len(s.queue) + 1, done: make(chan int, 1)}
		s.workers = append(s.workers, w)
		s.queue <- w
	}
	services = append(services, s)
	return s
}

//Cmd get command (first argument) when running app
func Cmd() string {
	if len(os.Args) > 1 {
		return strings.TrimSpace(os.Args[1])
	}
	return ``
}

//CmdInt get command (first argument) when running app
func CmdInt() int {
	cmd := Cmd()
	if cmd != `` {
		if i, e := strconv.Atoi(cmd); e == nil {
			return i
		}
	}
	return 0
}

//Arg get arguments after command
func Arg(idx int) string {
	if ArgExist(idx) {
		return strings.TrimSpace(os.Args[idx+2])
	}
	return ``
}

//ArgInt get arguments after command
func ArgInt(idx int) int {
	if i, e := strconv.Atoi(Arg(idx)); e == nil {
		return i
	}
	return 0
}

//ArgExist ...
func ArgExist(idx int) bool {
	return len(os.Args)-2 > idx
}
