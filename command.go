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

	debugLog = log.Println
	infoLog  = log.Println
	errorLog = log.Println

	filteredArgs []string

	pidfile = ``
)

// Log ...
func Log(d, i, e func(...interface{})) {
	debugLog = d
	infoLog = i
	errorLog = e
}

// Filename get application name
func Filename() string {
	_, file := filepath.Split(os.Args[0])
	return file
}

// ProcessExist return error when not exist (linux only)
func ProcessExist() error {
	data, e := ioutil.ReadFile(pidfile)
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

// SetPidFile ...
func SetPidFile(name string) {
	pidfile = name
}

// Start ...
func Start() error {
	if exitSignal != nil {
		return errors.New(`services already running`)
	}
	for _, s := range services {
		s.start()
	}
	exitSignal = make(chan os.Signal, 1)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGQUIT)
	go func() {
		<-exitSignal
		Stop()
	}()
	return nil
}

// Stop ...
func Stop() {
	for _, s := range services {
		s.stop()
	}
}

// Wait ...
func Wait() {
	wg := sync.WaitGroup{}
	wg.Add(len(services))
	for _, s := range services {
		go func(s *Service) {
			defer wg.Done()
			<-s.doneCh
		}(s)
	}
	wg.Wait()
	os.Remove(pidfile)
}

// Add ...
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

// Cmd get command (first argument) when running app
func Cmd() string {
	if len(os.Args) > 1 {
		return strings.TrimSpace(os.Args[1])
	}
	return ``
}

// CmdInt get command (first argument) when running app
func CmdInt() int {
	cmd := Cmd()
	if cmd != `` {
		if i, e := strconv.Atoi(cmd); e == nil {
			return i
		}
	}
	return 0
}

// Get get argument with value.
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

// Get get argument with value.
// Ex:
// --config=path.cfg
// Get(`config`) = path.cfg
func GetInt(key string) int {
	val := Get(key)
	if i, e := strconv.Atoi(val); e == nil {
		return i
	}
	return 0
}

// Args get all arguments after command
func Args() []string {
	if filteredArgs == nil {
		for _, arg := range os.Args[2:] {
			if !strings.HasPrefix(arg, `--`) {
				filteredArgs = append(filteredArgs, arg)
			}
		}
	}
	return filteredArgs
}

// Arg get arguments after command
func Arg(idx int) string {
	if ArgExist(idx) {
		return strings.TrimSpace(filteredArgs[idx])
	}
	return ``
}

// ArgInt get arguments after command
func ArgInt(idx int) int {
	if i, e := strconv.Atoi(Arg(idx)); e == nil {
		return i
	}
	return 0
}

// ArgExist ...
func ArgExist(idx int) bool {
	return len(Args()) > idx
}
