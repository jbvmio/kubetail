package cmd

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	v1 "k8s.io/api/core/v1"
	rest "k8s.io/client-go/rest"
)

// PodLogger implements PodInterface
type PodLogger struct {
	pod      k8sSelfLink
	req      *rest.Request
	logPool  *sync.Pool
	stopChan chan struct{}
}

// PodLogs contains logs from a pod.
type PodLogs struct {
	pod  string
	logs []byte
}

type logWatch struct {
	pod    string
	stream io.ReadCloser
	logs   chan PodLogs
}

func (l *logWatch) Watch() {
	for {
		logs := make([]byte, maxSize)
		read, err := l.stream.Read(logs)
		if err != nil {
			if strings.Contains(err.Error(), `response body closed`) {
				return
			}
			if strings.Contains(err.Error(), `EOF`) {
				fmt.Println(l.pod, err.Error())
				return
			}
			log.Fatalf("Error buffering log stream: %v\n", err)
		}
		podLogs := PodLogs{
			pod:  l.pod,
			logs: logs[:read],
		}
		l.logs <- podLogs
	}
}

func (l *logWatch) Logs() chan PodLogs {
	return l.logs
}

// StartStream starts collection of Pod Logs
func (p *PodLogger) StartStream() {
	defer wg.Done()
	fmt.Println(p.pod.Name, "Starting Stream")
	stream, err := p.req.Stream()
	if err != nil {
		log.Fatalf("Error obtaining log stream: %v\n", err)
	}
	podLogs := logWatch{
		pod:    p.pod.Name,
		stream: stream,
		logs:   make(chan PodLogs),
	}
	go podLogs.Watch()
logLoop:
	for {
		select {
		case <-p.stopChan:
			stream.Close()
			break logLoop
		case podLog := <-podLogs.Logs():
			p.logPool.Put(&podLog)
		}
	}
	fmt.Println(p.pod.Name, "Stopped")
}

func logDefaults(tl int64) *v1.PodLogOptions {
	lines := tl
	o := v1.PodLogOptions{}
	o.Follow = true
	o.TailLines = &lines
	return &o
}

func regexLogs(logPool, regexPool *sync.Pool, regexOrder []RegexMaker, stopChan chan struct{}) {
	defer wg.Done()
processLoop:
	for {
		select {
		case <-stopChan:
			break processLoop
		default:
			pod := logPool.Get()
			if pod != nil {
				p := pod.(*PodLogs)
				var parsedLogs []byte
				lines := bytes.Split(p.logs, []byte{10})
				for _, regex := range regexOrder {
					find := regex.GetRegex()
					switch regex.(*RegexList).Type {
					case Black:
						for _, line := range lines {
							l := bytes.TrimSpace(line)
							if !find.MatchString(fmt.Sprintf("%s", l)) && len(l) > 0 {
								l = append(l, byte(10))
								parsedLogs = append(parsedLogs, l...)
							}
						}
					case White:
						for _, line := range lines {
							l := bytes.TrimSpace(line)
							if find.MatchString(fmt.Sprintf("%s", l)) && len(l) > 0 {
								l = append(l, byte(10))
								parsedLogs = append(parsedLogs, l...)
							}
						}
					}
				}
				if len(parsedLogs) > 0 {
					regexPool.Put(&PodLogs{
						pod:  p.pod,
						logs: parsedLogs,
					})
				}
			} else {
				backoff(500)
			}
		}
	}
}

func processLogs(pool *sync.Pool, headers bool, stopChan chan struct{}) {
	defer wg.Done()
processLoop:
	switch {
	case headers:
		for {
			select {
			case <-stopChan:
				break processLoop
			default:
				pod := pool.Get()
				if pod != nil {
					p := pod.(*PodLogs)
					head := color.YellowString("[%v]", p.pod)
					fmt.Printf("%v\n%s", head, p.logs)
				} else {
					backoff(500)
				}
			}
		}
	default:
		for {
			select {
			case <-stopChan:
				break processLoop
			default:
				pod := pool.Get()
				if pod != nil {
					p := pod.(*PodLogs)
					fmt.Printf("%s", p.logs)
				} else {
					backoff(500)
				}
			}
		}
	}
}

func backoff(millis int) {
	durationMillisecond := 1 * time.Millisecond
	timeout := durationMillisecond * time.Duration(millis)
	time.Sleep(timeout)
	return
}
