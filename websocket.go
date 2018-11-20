package netserver
import(
	"net"
	"github.com/qianlidongfeng/log"
	"time"
	"strings"
	"bufio"
	"errors"
	"fmt"
	"bytes"
	"net/url"
	"crypto/sha1"
	"encoding/base64"
)

type WebsocketServer struct{
	listener net.Listener
	OnAccept func (conn net.Conn)
}

func NewWebsocketServer() WebsocketServer{
	return WebsocketServer{}
}

func (this *WebsocketServer) Listen(listenAddr string) error{
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

func (this *WebsocketServer)accept() error{
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
		err = this.handshake(conn)
		if err != nil{
			fmt.Fprintf(conn, "HTTP/1.1 "+"400 Bad Request\r\nContent-Type: text/plain; charset=utf-8\r\nConnection"+err.Error())
			conn.Close()
			continue
		}
		this.OnAccept(conn)
	}
}

func (this *WebsocketServer) handshake(conn net.Conn) error{
	b:=bufio.NewReader(conn)
	s,_,err:=b.ReadLine()
	if err != nil{
		return err
	}
	method, requestURI, _, ok:=parseRequestLine(string(s))
	if !ok {
		return errors.New("malformed HTTP request:" + string(s))
	}
	if method != "GET"{
		return errors.New("invalid method:" + string(s))
	}
	header,err:= readHeader(b)
	if err != nil{
		return err
	}
	if strings.ToLower(header["Upgrade"]) != "websocket"||
		!strings.Contains(strings.ToLower(header["Connection"]), "upgrade"){
		return errors.New("not websocket protocol")
	}
	key:= header["Sec-WebSocket-Key"]
	if key==""{
		return errors.New("mismatch challenge/response")
	}
	if header["Sec-WebSocket-Version"] != "13"{
		return errors.New("missing or bad WebSocket Version")
	}
	var protocols []string
	protocol :=strings.TrimSpace(header["Sec-WebSocket-Protocol"])
	if protocol != "" {
		p := strings.Split(protocol, ",")
		for i := 0; i < len(p); i++ {
			protocols = append(protocols, strings.TrimSpace(p[i]))
		}
	}
	origin:= header["Origin"]
	if origin == "" {
		return errors.New("null origin")
	}
	_,err=url.ParseRequestURI(origin)
	if err != nil{
		return errors.New("error origin")
	}
	_,err=url.Parse("ws://"+header["Host"]+requestURI)
	if err!=nil{
		return errors.New("path forbidden")
	}
	akey, err := getNonceAccept([]byte(key))
	if err != nil{
		return err
	}
	if len(protocols) > 0 {
		if len(protocols) != 1 {
			return errors.New("missing or bad WebSocket-Protocol")
		}
	}
	conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n"))
	conn.Write([]byte("Upgrade: websocket\r\n"))
	conn.Write([]byte("Connection: Upgrade\r\n"))
	conn.Write([]byte("Sec-WebSocket-Accept: " + string(akey) + "\r\n"))
	if len(protocols) > 0 {
		conn.Write([]byte("Sec-WebSocket-Protocol: " + protocols[0] + "\r\n"))
	}
	conn.Write([]byte("\r\n"))
	return nil
}

func (this *WebsocketServer) Close(){
	this.listener.Close()
}

func parseRequestLine(line string) (method, requestURI, proto string, ok bool) {
	s1 := strings.Index(line, " ")
	s2 := strings.Index(line[s1+1:], " ")
	if s1 < 0 || s2 < 0 {
		return
	}
	s2 += s1 + 1
	return line[:s1], line[s1+1 : s2], line[s2+1:], true
}

func readHeader(reader *bufio.Reader) (header map[string]string,err error){
	err = nil
	var kv []byte
	header=make(map[string]string)
	for{
		kv,_,err=reader.ReadLine()
		if err != nil{
			return
		}
		if len(kv) == 0 {
			break
		}
		i :=bytes.IndexByte(kv,':')
		if i<0{
			err = errors.New("malformed header line: " + string(kv))
			return
		}
		endKey := i
		for endKey > 0 && kv[endKey-1] == ' ' {
			endKey--
		}
		i++
		for i < len(kv) && (kv[i] == ' ' || kv[i] == '\t') {
			i++
		}
		value := string(kv[i:])
		header[string(kv[:endKey])]=value
	}
	return
}

func getNonceAccept(nonce []byte) (expected []byte, err error) {
	h := sha1.New()
	if _, err = h.Write(nonce); err != nil {
		return
	}
	if _, err = h.Write([]byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")); err != nil {
		return
	}
	expected = make([]byte, 28)
	base64.StdEncoding.Encode(expected, h.Sum(nil))
	return
}