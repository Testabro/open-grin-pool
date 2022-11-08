package main

// http rpc server
import (
	"context"
	"encoding/json"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type stratumServer struct {
	db   *database
	ln   net.Listener
	conf *config
}

type stratumRequest struct {
	ID      string                 `json:"id"`
	JsonRpc string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}

type stratumResponse struct {
	ID      string                 `json:"id"`
	JsonRpc string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Result  interface{}            `json:"result"`
	Error   map[string]interface{} `json:"error"`
}

type minerSession struct {
	login      string
	agent      string
	difficulty int64
	ctx        context.Context
}

func (ms *minerSession) hasNotLoggedIn() bool {
	return ms.login == ""
}

func (ms *minerSession) handleMethod(res *stratumResponse, db *database) {
	switch res.Method {
	case "status":
		if ms.login == "" {
			log.Warning("recv status detail before login")
			break
		}
		result, _ := res.Result.(map[string]interface{})
		db.setMinerAgentStatus(ms.login, ms.agent, ms.difficulty, result)

		break
	case "submit":
		if res.Error != nil {
			log.Warning(ms.login, "'s share has err: ", res.Error)
			break
		}
		detail, ok := res.Result.(string)
		log.Info(ms.login, " has submit a ", detail, " share")
		if ok {
			db.putShare(ms.login, ms.agent, ms.difficulty)
			if strings.Contains(detail, "block") {
				blockHash := strings.Trim(detail, "block - ")
				db.putBlockHash(blockHash)
				log.Warning("block ", blockHash, " has been found by ", ms.login)
			}
		}
		break
	}
}

func callStatusPerInterval(ctx context.Context, nc *nodeClient) {
	statusReq := &stratumRequest{
		ID:      "0",
		JsonRpc: "2.0",
		Method:  "status",
		Params:  nil,
	}

	ch := time.Tick(10 * time.Second)
	enc := json.NewEncoder(nc.c)

	for {
		select {
		case <-ch:
			err := enc.Encode(statusReq)
			if err != nil {
				log.Error(err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (ss *stratumServer) handleConn(conn net.Conn) {
	log.Info("new conn from ", conn.RemoteAddr())
	session := &minerSession{difficulty: int64(ss.conf.Node.Diff)}
	defer conn.Close()
	var login string
	nc := initNodeStratumClient(ss.conf)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go nc.registerHandler(ctx, func(sr json.RawMessage) {
		enc := json.NewEncoder(conn)
		err := enc.Encode(sr)
		if err != nil {
			log.Error(err)
		}

		// internal record
		var res stratumResponse
		_ = json.Unmarshal(sr, &res) // suppress the err

		session.handleMethod(&res, ss.db)
	})
	defer nc.close()

	dec := json.NewDecoder(conn)
	for {
		var jsonRaw json.RawMessage
		var clientReq stratumRequest

		err := dec.Decode(&jsonRaw)
		if err != nil {
			opErr, ok := err.(*net.OpError)
			if ok {
				if opErr.Err.Error() == syscall.ECONNRESET.Error() {
					return
				}
			} else {
				log.Error(err)
			}
		}

		if len(jsonRaw) == 0 {
			return
		}

		err = json.Unmarshal(jsonRaw, &clientReq)
		if err != nil {
			// log.Error(err)
			continue
		}

		log.Info(conn.RemoteAddr(), " sends a ", clientReq.Method, " request:", string(jsonRaw))

		switch clientReq.Method {
		case "login":
			login, _ = clientReq.Params["login"].(string)

			pass, _ := clientReq.Params["pass"].(string)

			agent, _ := clientReq.Params["agent"].(string)

			login = strings.TrimSpace(login)
			pass = strings.TrimSpace(pass)
			agent = strings.TrimSpace(agent)

			if agent == "" {
				agent = "NoNameMiner" + strconv.FormatInt(rand.Int63(), 10)
			}

			switch ss.db.verifyMiner(login, pass) {
			case wrongPassword:
				log.Warning(login, " has failed to login")
				login = ""
				_, _ = conn.Write([]byte(`{  
   "id":"5",
   "jsonrpc":"2.0",
   "method":"login",
   "error":{  
      "code":-32500,
      "message":"login incorrect"
   }
}`))

			case noPassword:
				ss.db.registerMiner(login, pass, "")
				log.Info(login, " has registered in")

			case correctPassword:

			}

			session.login = login
			session.agent = agent

			requireCallStatus := true
			for _, omitSubstr := range ss.conf.StratumServer.OmitAgentStatus {
				if strings.Contains(agent, omitSubstr) {
					requireCallStatus = false
					break
				}
			}
			if requireCallStatus {
				go callStatusPerInterval(ctx, nc)
			}

			log.Info(session.login, "'s ", agent, " has logged in")
			_ = nc.enc.Encode(jsonRaw)

		default:
			if session.hasNotLoggedIn() {
				log.Warning(login, " has not logged in")
			}

			_ = nc.enc.Encode(jsonRaw)
		}
	}
}

func initStratumServer(db *database, conf *config) {
	ip := net.ParseIP(conf.StratumServer.Address)
	addr := &net.TCPAddr{
		IP:   ip,
		Port: conf.StratumServer.Port,
	}
	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	log.Warning("listening on ", conf.StratumServer.Port)

	ss := &stratumServer{
		db:   db,
		ln:   ln,
		conf: conf,
	}

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			log.Error(err)
		}

		go ss.handleConn(conn)
	}
}
