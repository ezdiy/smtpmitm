package main

import (
	"log"
	"net"
	"os"
)
import "github.com/ezdiy/smtpmitm"

func main() {
	l, _ := net.Listen("tcp", ":25")
	for {
		c, _ := l.Accept()
		s, _ := net.Dial("tcp", "smtp.seznam.cz:25")
		mitm := smtpmitm.Session{Tarpit:1}
		mitm.Server.Set(s, 0)
		mitm.Client.Set(c, 0)
		mitm.Server.Logger = log.New(os.Stderr, "SERVER ", log.LstdFlags)
		mitm.Client.Logger = log.New(os.Stderr, "CLIENT ", log.LstdFlags)
		mitm.MITM()
	}
}
