package runner

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/matematik7/didcj/config"
	"github.com/matematik7/didcj/models"
	"github.com/matematik7/didcj/utils"
	"github.com/pkg/errors"
)

const (
	SEND    = 0
	RECEIVE = 1
	DEBUG   = 2
)

const (
	INITIALIZED = 0
	RUNNING     = 1
	DONE        = 2
	ERROR       = 3
)

const MB = 1024 * 1024

type Runner struct {
	nodeid  int
	servers []*models.Server

	config *config.Config

	port string

	cmd *exec.Cmd

	stderr io.ReadCloser
	stdout io.ReadCloser
	stdin  io.WriteCloser

	tcpListener net.Listener

	receiveChannels []chan []byte

	status    int
	msgsMutex *sync.Mutex
	report    *models.Report

	startTime    time.Time
	timeoutTimer *time.Timer
}

func New() *Runner {
	return &Runner{
		port: "3456",
	}
}

func (r *Runner) Init() error {
	var err error

	r.nodeid, err = utils.Nodeid()
	if err != nil {
		return errors.Wrap(err, "runner.init")
	}

	r.servers, err = utils.ServerList()
	if err != nil {
		return errors.Wrap(err, "runner.init")
	}

	r.receiveChannels = make([]chan []byte, len(r.servers))

	return nil
}

func (r *Runner) Start(cfg *config.Config) {
	r.config = cfg
	go r.start()
}

func (r *Runner) Stop() {
	r.error(fmt.Errorf("Received stop"), "stop")
}

func (r *Runner) Status() int {
	return r.status
}

func (r *Runner) Report() *models.Report {
	r.msgsMutex.Lock()
	defer r.msgsMutex.Unlock()

	return r.report
}

func (r *Runner) start() {
	var err error

	r.msgsMutex = &sync.Mutex{}
	r.status = RUNNING
	r.report = &models.Report{
		Ip:       r.servers[r.nodeid].Ip.String(),
		Messages: make([]string, 0, 100),
	}

	for i := range r.receiveChannels {
		r.receiveChannels[i] = make(chan []byte, 10)
	}

	r.tcpListener, err = net.Listen("tcp", fmt.Sprintf(":%s", r.port))
	if err != nil {
		r.error(err, "runner.start")
		return
	}
	defer r.tcpListener.Close()
	go r.tcpListen()

	appFile, err := utils.FindFileBasename("app")
	if err != nil {
		r.error(err, "runner.start")
		return
	}

	r.cmd = exec.Command("./" + appFile + ".app")

	r.stderr, err = r.cmd.StderrPipe()
	if err != nil {
		r.error(err, "runner.start")
		return
	}
	defer r.stderr.Close()
	// stdout is closed in handleStdout
	r.stdout, err = r.cmd.StdoutPipe()
	if err != nil {
		r.error(err, "runner.start")
		return
	}
	r.stdin, err = r.cmd.StdinPipe()
	if err != nil {
		r.error(err, "runner.start")
		return
	}
	defer r.stdin.Close()

	go r.handleStdout()

	time.Sleep(time.Millisecond * 10)

	r.timeoutTimer = time.AfterFunc(time.Second*time.Duration(r.config.MaxTimeSeconds), func() {
		r.error(fmt.Errorf("Timeout!"), "runner.start")
	})
	r.startTime = time.Now()

	err = r.cmd.Start()
	if err != nil {
		r.error(err, "runner.start")
		return
	}

	for {
		buffer := make([]byte, 1)
		n, err := r.stderr.Read(buffer)
		if err == io.EOF {
			break
		} else if err != nil {
			r.error(err, "runner.start")
			return
		} else if n == 1 {
			if buffer[0] == RECEIVE {
				source, err := r.readInt(r.stderr)
				if err != nil {
					r.error(err, "runner.start.receive")
					return
				}
				data := <-r.receiveChannels[source]
				r.stdin.Write(r.formatInt(len(data)))
				r.stdin.Write(r.formatInt(source))
				r.stdin.Write(data)
			} else if buffer[0] == SEND {
				if r.report.SendCount >= r.config.MaxMsgsPerNode {
					r.error(fmt.Errorf("Too many msgs!"), "runner.start.send")
					return
				}
				target, err := r.readInt(r.stderr)
				if err != nil {
					r.error(err, "runner.start.send")
					return
				}
				length, err := r.readInt(r.stderr)
				if err != nil {
					r.error(err, "runner.start.send")
					return
				}
				if length > r.config.MaxMsgSizeMb*MB {
					r.error(fmt.Errorf("Msg too big, size: %d", length), "runner.start.send")
					return
				}
				if length > r.report.LargestMsg {
					r.report.LargestMsg = length
				}
				msg := make([]byte, length)
				_, err = io.ReadFull(r.stderr, msg)
				if err != nil {
					r.error(err, "runner.start.send")
					return
				}
				conn, err := net.Dial("tcp", fmt.Sprintf("%s:%s",
					r.servers[target].Ip.String(),
					r.port,
				))
				if err != nil {
					r.error(err, "runner.start.send")
					return
				}
				conn.Write(r.formatInt(r.nodeid))
				conn.Write(msg)
				conn.Close()
				r.report.SendCount++
			} else if buffer[0] == DEBUG {
				length, err := r.readInt(r.stderr)
				if err != nil {
					r.error(err, "runner.start.debug")
					return
				}
				msg := make([]byte, length)
				_, err = io.ReadFull(r.stderr, msg)
				if err != nil {
					r.error(err, "runner.start.debug")
				}
				r.debug(string(msg))
			} else {
				r.error(fmt.Errorf("Invalid buffer: %v", buffer), "runner.start")
			}
		}
	}

	err = r.cmd.Wait()
	r.report.RunTime = time.Now().Sub(r.startTime).Nanoseconds()
	r.timeoutTimer.Stop()
	r.status = DONE
	if err != nil {
		r.error(err, "runner.start")
		return
	}
}

func (r *Runner) tcpListen() {
	for {
		conn, err := r.tcpListener.Accept()
		if err != nil {
			break
		}

		source, err := r.readInt(conn)
		if err != nil {
			r.error(err, "runner.tcplisten")
			continue
		}

		data, err := ioutil.ReadAll(conn)
		if err != nil {
			r.error(err, "runner.tcplisten")
			continue
		}

		if len(r.receiveChannels[source]) > 0 {
			log.Printf("Message from %d when %d already in queue!", source, len(r.receiveChannels[source]))
		}
		r.receiveChannels[source] <- data
		conn.Close()
	}
}

func (r *Runner) error(err error, wrap string) {
	if r.cmd != nil && r.cmd.Process != nil && r.status == RUNNING {
		err := r.cmd.Process.Kill()
		if err != nil {
			r.debug(fmt.Sprintf("Could not kill process: %v", err))
		}
	}
	r.debug(errors.Wrap(err, wrap).Error())
	r.status = ERROR
}

func (r *Runner) debug(msg string) {
	r.msgsMutex.Lock()
	defer r.msgsMutex.Unlock()

	r.report.Messages = append(r.report.Messages, msg)
}

func (r *Runner) readInt(reader io.Reader) (int, error) {
	buf := make([]byte, 4)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return 0, errors.Wrap(err, "readint")
	}

	value := 0
	for i, b := range buf {
		value |= int(b) << uint(8*i)
	}

	return value, nil
}

func (r *Runner) formatInt(value int) []byte {
	data := make([]byte, 4)
	for i := 0; i < 4; i++ {
		data[i] = byte(0xff & (value >> uint(8*i)))
	}
	return data
}

func (r *Runner) handleStdout() {
	bReader := bufio.NewReader(r.stdout)
	defer r.stdout.Close()
	for {
		msg, err := bReader.ReadString('\n')
		if len(msg) != 0 {
			r.debug("stdout: " + msg[:len(msg)-1])
		}

		if err != nil && (err == io.EOF || err == os.ErrClosed || err.Error() == "read |0: file already closed") {
			return
		} else if err != nil {
			r.error(err, "handlestdout")
			return
		}
	}
}
