package netserver

import (
	"net"
	"time"
	"github.com/qianlidongfeng/log"
	"io"
)
type TcpServer struct{
	listener net.Listener
	OnAccept func (conn net.Conn)
}

func NewTcpServer() TcpServer{
	return TcpServer{
		OnAccept:func (conn net.Conn){},
	}
}


func (this *TcpServer) Listen(listenAddr string) error{
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	this.listener = listener
	err=this.accept()
	if err != nil{
		return err
	}
	return nil
}

func (this *TcpServer)accept() error{
	var tempDelay time.Duration
	for{
		conn, err := this.listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Warn(err)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0
		this.OnAccept(conn)
	}
}


func (this *TcpServer) Close(){
	this.listener.Close()
}

func WriteFull(writer io.Writer,data []byte) error{
	length:=len(data)
	var nn int
	for{
		n,err:=writer.Write(data[nn:])
		if err != nil{
			return err
		}
		nn += n
		if nn>= length{
			break
		}
	}
	return nil
}