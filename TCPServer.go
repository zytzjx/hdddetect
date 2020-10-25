package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/widuu/gojson"
)

var (
	maxRead  = 4096
	msgError = []byte(`{"result":"error"}`)
)

func checkError(err error, info string) {
	if err != nil {
		fmt.Errorf("ERROR: " + info + " " + err.Error())
		panic("ERROR: " + info + " " + err.Error()) // terminate program
	}
}

func StartTcpServer() {

	hostAndPort := ":12000"
	listener := initServer(hostAndPort)
	for {
		conn, err := listener.Accept()
		checkError(err, "Accept: ")
		go connectionHandler(conn)
	}

}
func initServer(hostAndPort string) *net.TCPListener {
	serverAddr, err := net.ResolveTCPAddr("tcp", hostAndPort)
	checkError(err, "Resolving address:port failed: '"+hostAndPort+"'")
	listener, err := net.ListenTCP("tcp", serverAddr)
	checkError(err, "ListenTCP: ")
	println("Listening to: ", listener.Addr().String())
	return listener
}
func connectionHandler(conn net.Conn) {
	connFrom := conn.RemoteAddr().String()
	println("Connection from: ", connFrom)
	for {
		var ibuf []byte = make([]byte, maxRead+1)
		length, err := conn.Read(ibuf[0:maxRead])
		ibuf[maxRead] = 0 // to prevent overflow
		switch err {
		case nil:
			handleMsg(conn, length, err, ibuf)
		default:
			goto DISCONNECT
		}
	}
DISCONNECT:
	err := conn.Close()
	println("Closed connection:", connFrom)
	checkError(err, "Close:")
}
func talktoclients(to net.Conn, msg []byte) {
	wrote, err := to.Write(msg)
	checkError(err, "Write: wrote "+string(wrote)+" bytes.")
}

func handleMsg(conn net.Conn, length int, err error, msg []byte) {
	if length > 0 {
		var bsend bool
		cmd := string(msg[:length])
		if strings.EqualFold(cmd, "print\n") {
			bsend = true
		} else {
			cmds := gojson.Json(cmd).Getdata()
			if val, ok := cmds["cmd"]; ok {
				if strings.EqualFold(val.(string), "print") {
					bsend = true
				}
			}
		}

		if bsend {
			ss := DetectData.String()
			talktoclients(conn, []byte(ss))
		} else {
			talktoclients(conn, msgError)
		}

	} else {
		talktoclients(conn, msgError)
	}
}
