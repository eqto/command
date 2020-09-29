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

//SetPidFile ...
func SetPidFile(name string) {
	pidfile = name
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

//Get get argument with value.
// Ex:
// --config=path.cfg
// Get(`config`) = path.cfg
func Get(key string) string {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, `--`+key+`=`) {
			return arg[strings.Index(arg, `=`)+1:]
		}
	}
	return ``
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
