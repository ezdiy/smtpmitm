package smtpmitm

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

var DefaultTimeout = time.Duration(30) * time.Second

type Stream struct {
	*textproto.Reader
	net.Conn
	Timeout time.Duration
	*log.Logger
}

func (s *Stream) Set(c net.Conn, timeout int) {
	s.Timeout = DefaultTimeout
	if timeout > 0 {
		s.Timeout = time.Duration(timeout) * time.Second
	}
	s.Conn = c
	s.Reader = textproto.NewReader(bufio.NewReader(io.LimitReader(c, 1000)))
}

// One currently relayed session. You have to set up Server and Client streams up front before calling MITM.
type Session struct {
	Tarpit int
	Server, Client Stream
}

// Read arbitrary line, with optional timeout.
func (s *Stream) ReadLine() (line string) {
	s.SetReadDeadline(time.Now().Add(s.Timeout))
	line, err := s.Reader.ReadLine()
	if err != nil {
		panic(err)
	}
	if s.Logger != nil {
		s.Logger.Println("<< " + line)
	}
	return
}

// Read command from a client
func (s *Stream) ReadCommand() (cmd, arg string) {
	ln := s.ReadLine()
	parts := append(strings.SplitN(ln, " ", 2), "")
	return parts[0], parts[1]
}

func (s *Stream) SendCommand(cmd, arg string) {
	if arg != "" {
		s.SendLine(cmd + " " + arg)
	} else {
		s.SendLine(cmd)
	}
}

// Read a (possibly multi-line) response from a server. Like readresponse, but more permissive.
func (s *Stream) ReadReply() (code int, lines[] string) {
	code = -1
	for {
		ln := s.ReadLine()
		cc, err := strconv.Atoi(ln[0:3])
		if err != nil {
			panic(err)
		}
		if code == -1 {
			code = cc
		} else if cc != code {
			panic("wrong code")
		}
		l := ""
		if len(ln) > 4 {
			l = ln[4:]
		}
		lines = append(lines,l)
		if len(ln) == 3 || ln[3] == ' ' {
			break
		}
	}
	return
}

// Send arbitrary line
func (s *Stream) SendLine(ln string) {
	if s.Logger != nil {
		s.Logger.Println(">> " + ln)
	}
	s.SetWriteDeadline(time.Now().Add(s.Timeout))
	_, err := s.Write([]byte(ln + "\r\n"))
	if err != nil {
		panic(err)
	}
}

// Send (possible multi-line) server response
func (s *Stream) SendReply(code int, lns []string) {
	for i, v := range lns {
		sep := '-'
		if i == len(lns)-1 {
			sep = ' '
		}
		s.SendLine(fmt.Sprintf("%03d%c%s", code, sep, v))
	}
}

func (s *Session) MITM() {
	defer func() {
		err := recover()
		if err != nil {
			log.Println(err)
		}
		s.Server.Close()
		s.Client.Close()
	}()

	// Now enter the command loop
	for {
		reply, lines := s.Server.ReadReply()

		switch reply {
		case 220:
			if s.Tarpit > 0 {
				for _, l := range lines {
					s.Client.SendLine("220-" + l)
				}
				var b[1]byte
				s.Client.SetReadDeadline(time.Now().Add(time.Duration(s.Tarpit) * time.Second))
				got, err := s.Client.Conn.Read(b[:])
				if got > 0 && err == nil {
					s.Client.SetReadDeadline(time.Time{})
					s.Server.Close()
					// got premature data from spammer, enter the tarpit
					io.Copy(ioutil.Discard, s.Client.Conn)
					return
				}
				s.Client.SendLine("220 ")
			} else {
				s.Client.SendReply(reply, lines)
			}
		case 250:

			for i := 0; i < len(lines); i++ {
				v := lines[i]
				//log.Println(v)
				if i > 0 && (v == "STARTTLS" || v == "PIPELINING" || v == "CHUNKING" || v == "REQUIRETLS") {
					copy(lines[i:], lines[i+1:])
					lines = lines[:len(lines)-1]
					i--
				}
			}
			s.Client.SendReply(reply, lines)
		case 334:
			s.Client.SendReply(reply, lines)
			s.Server.SendLine(s.Client.ReadLine())
			continue
		case 354:
			s.Client.SendReply(reply, lines)
			for {
				ln := s.Client.ReadLine()
				s.Server.SendLine(ln)
				if ln == "." {
					break
				}
			}
			continue
		default:
			s.Client.SendReply(reply, lines)
			if reply == 221 {
				return
			}
		}
nextCommand:
		cmd, arg := s.Client.ReadCommand()
		if cmd == "STARTTLS" {
			s.Client.SendLine("454 TLS not available")
			goto nextCommand
		}
		s.Server.SendCommand(cmd, arg)
	}
}

