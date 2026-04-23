package tcp

import (
	"bufio"
	"io"
	"net"
	"strings"
)

type Logger interface {
	Trace(string, ...any)
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}

type TCPServer struct {
	addr   string
	logger Logger
}

func New(addr string, logger Logger) *TCPServer {
	return &TCPServer{
		addr:   addr,
		logger: logger,
	}
}

func (server *TCPServer) Start() error {
	ln, err := net.Listen("tcp", server.addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	server.logger.Info("TCP server started on %s", server.addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			server.logger.Error("Failed to accept connection: %v", err)
			continue
		}

		go server.handleConnection(conn)
	}
}

func (server *TCPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				server.logger.Error("Failed to read from connection: %v", err)
			}
			return
		}

		line = strings.TrimRight(line, "\r\n")
		server.logger.Trace("Got `%s` from client", line)
		result, err := HandleCommandStr(line)

		if err != nil {
			_, err = writer.WriteString("ERROR: " + err.Error() + "\n")
		} else {
			_, err = writer.WriteString("SUCCESS: " + result + "\n")
		}

		if err != nil {
			server.logger.Error("Failed to write to connection: %v", err)
			return
		}

		err = writer.Flush()
		if err != nil {
			server.logger.Error("Failed to flush writer: %v", err)
			return
		}
	}
}
