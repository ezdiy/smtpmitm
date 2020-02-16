package main

import (
	"net"
)
import "github.com/ezdiy/smtpmitm"

func main() {
	l, _ := net.Listen("tcp", ":25")
	cfg := smtpmitm.Config{
		Tarpit220:      7,
		TarpitBanner:   "$ ESMTP Postfix",
	}
	for {
		c, _ := l.Accept()
		s, _ := net.Dial("tcp", "gmail-smtp-in.l.google.com:25")
		mitm := smtpmitm.Session{Config:&cfg}
		mitm.Server.Set(s, 0)
		mitm.Client.Set(c, 0)
		//mitm.Server.Logger = log.New(os.Stderr, "SERVER ", log.LstdFlags)
		//mitm.Client.Logger = log.New(os.Stderr, "CLIENT ", log.LstdFlags)
		mitm.MITM()
	}
}
