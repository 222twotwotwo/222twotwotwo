package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	raft "raft"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "server":
		runServer(os.Args[2:])
	case "get":
		runClient(os.Args[2:], "get")
	case "put":
		runClient(os.Args[2:], "put")
	case "append":
		runClient(os.Args[2:], "append")
	case "status":
		runStatus(os.Args[2:])
	case "demo":
		runDemo(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `用法:
  raft server -id 0 -addr 127.0.0.1:9001 -peers 0=127.0.0.1:9001,1=127.0.0.1:9002,2=127.0.0.1:9003 -data data/n0
  raft get    -servers 127.0.0.1:9001,127.0.0.1:9002,127.0.0.1:9003 -key foo
  raft put    -servers ... -key foo -value bar
  raft append -servers ... -key foo -value bar
  raft status -servers ...
  raft demo   [-nodes 3]
`)
}

func runServer(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	id := fs.Int("id", 0, "本节点编号")
	addr := fs.String("addr", "127.0.0.1:9001", "本节点监听地址")
	peersFlag := fs.String("peers", "", "集群地址，形如 0=host:port,1=host:port")
	dataDir := fs.String("data", "", "状态文件目录")
	maxState := fs.Int("maxraftstate", 1<<20, "日志超过该字节数就打快照，0 表示不打")
	heartbeat := fs.Duration("heartbeat", 100*time.Millisecond, "心跳间隔")
	electionMin := fs.Duration("election-min", 300*time.Millisecond, "选举超时下限")
	electionMax := fs.Duration("election-max", 600*time.Millisecond, "选举超时上限")
	fs.Parse(args)

	addrs, err := parsePeers(*peersFlag)
	if err != nil {
		fatal(err)
	}
	if *id < 0 || *id >= len(addrs) {
		fatal(fmt.Errorf("节点编号 %d 不在集群范围 [0,%d) 内", *id, len(addrs)))
	}
	if *dataDir == "" {
		*dataDir = fmt.Sprintf("data/n%d", *id)
	}

	storage, err := raft.NewFileStorage(*dataDir)
	if err != nil {
		fatal(err)
	}
	applyCh := make(chan raft.ApplyMsg, 1024)
	rf, err := raft.NewRaft(raft.Config{
		ID:          *id,
		Peers:       addrs,
		Storage:     storage,
		Transport:   raft.NewNetTransport(addrs, 500*time.Millisecond),
		ApplyCh:     applyCh,
		Heartbeat:   *heartbeat,
		ElectionMin: *electionMin,
		ElectionMax: *electionMax,
	})
	if err != nil {
		fatal(err)
	}
	kv, err := raft.NewKVServer(rf, applyCh, *maxState)
	if err != nil {
		fatal(err)
	}

	server := rpc.NewServer()
	if err := server.RegisterName("Raft", rf); err != nil {
		fatal(err)
	}
	if err := server.RegisterName("KV", kv); err != nil {
		fatal(err)
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("节点 %d 监听 %s，集群 %v，数据目录 %s\n", *id, *addr, addrs, *dataDir)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go server.ServeConn(conn)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("正在退出")
	listener.Close()
	kv.Stop()
	rf.Kill()
}

func runClient(args []string, op string) {
	fs := flag.NewFlagSet(op, flag.ExitOnError)
	servers := fs.String("servers", "127.0.0.1:9001", "集群地址，逗号分隔")
	key := fs.String("key", "", "key")
	value := fs.String("value", "", "value")
	fs.Parse(args)

	if *key == "" {
		fatal(fmt.Errorf("必须指定 -key"))
	}
	clerk := raft.NewClerk(splitServers(*servers))
	switch op {
	case "get":
		fmt.Println(clerk.Get(*key))
	case "put":
		clerk.Put(*key, *value)
		fmt.Println("OK")
	case "append":
		clerk.Append(*key, *value)
		fmt.Println("OK")
	}
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	servers := fs.String("servers", "127.0.0.1:9001", "集群地址，逗号分隔")
	fs.Parse(args)

	fmt.Printf("%-4s %-10s %-6s %-6s %-8s %-8s %-8s %s\n",
		"id", "state", "term", "leader", "lastlog", "commit", "applied", "snapshot")
	for _, addr := range splitServers(*servers) {
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err != nil {
			fmt.Printf("%-4s %-10s\n", addr, "unreachable")
			continue
		}
		client := rpc.NewClient(conn)
		reply := raft.StatusReply{}
		err = client.Call("Raft.Status", &raft.StatusArgs{}, &reply)
		client.Close()
		if err != nil {
			fmt.Printf("%-4s %-10s %s\n", addr, "error", err)
			continue
		}
		fmt.Printf("%-4d %-10s %-6d %-6d %-8d %-8d %-8d %d\n",
			reply.ID, reply.State, reply.Term, reply.Leader,
			reply.LastLogIndex, reply.CommitIndex, reply.LastApplied, reply.SnapshotIndex)
	}
}

// runDemo 在本进程内起一个三节点集群，展示选主、读写和故障切换。
func runDemo(args []string) {
	fs := flag.NewFlagSet("demo", flag.ExitOnError)
	nodes := fs.Int("nodes", 3, "节点数")
	fs.Parse(args)
	if *nodes < 1 {
		fatal(fmt.Errorf("节点数必须为正"))
	}

	net := raft.NewLocalNetwork()
	net.SetDelay(2 * time.Millisecond)
	peers := make([]string, *nodes)
	for i := range peers {
		peers[i] = fmt.Sprintf("mem-%d", i)
	}
	rafts := make([]*raft.Raft, *nodes)
	kvs := make([]*raft.KVServer, *nodes)
	for i := 0; i < *nodes; i++ {
		applyCh := make(chan raft.ApplyMsg, 256)
		rf, err := raft.NewRaft(raft.Config{
			ID:          i,
			Peers:       peers,
			Storage:     raft.NewMemoryStorage(),
			Transport:   net.Transport(i),
			ApplyCh:     applyCh,
			Heartbeat:   50 * time.Millisecond,
			ElectionMin: 200 * time.Millisecond,
			ElectionMax: 400 * time.Millisecond,
		})
		if err != nil {
			fatal(err)
		}
		kv, err := raft.NewKVServer(rf, applyCh, 0)
		if err != nil {
			fatal(err)
		}
		net.Add(i, rf)
		rafts[i] = rf
		kvs[i] = kv
	}
	defer func() {
		for i := range rafts {
			if kvs[i] != nil {
				kvs[i].Stop()
			}
			if rafts[i] != nil {
				rafts[i].Kill()
			}
		}
	}()

	leader := waitLeader(rafts)
	fmt.Printf("选举完成：leader=%d term=%d\n", leader, termOf(rafts[leader]))

	client := &demoClient{id: time.Now().UnixNano()}
	client.put(kvs, "x", "1")
	client.append(kvs, "x", "2")
	fmt.Printf("put x=1 / append x=2 后，get x => %q\n", client.get(kvs, "x"))

	fmt.Printf("杀掉 leader %d\n", leader)
	net.Remove(leader)
	kvs[leader].Stop()
	rafts[leader].Kill()
	rafts[leader] = nil
	kvs[leader] = nil
	leader = waitLeader(rafts)
	fmt.Printf("重新选主：leader=%d term=%d\n", leader, termOf(rafts[leader]))
	fmt.Printf("换届后读 x => %q\n", client.get(kvs, "x"))

	client.append(kvs, "x", "3")
	fmt.Printf("新 leader 上 append x=3，get x => %q\n", client.get(kvs, "x"))
}

type demoClient struct {
	id  int64
	seq int
}

func (d *demoClient) get(kvs []*raft.KVServer, key string) string {
	d.seq++
	args := &raft.GetArgs{Key: key, Client: d.id, Seq: d.seq}
	for attempt := 0; attempt < 400; attempt++ {
		for _, kv := range kvs {
			if kv == nil {
				continue
			}
			reply := raft.GetReply{}
			if err := kv.Get(args, &reply); err != nil {
				continue
			}
			if reply.Err == "" {
				return reply.Value
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	fatal(fmt.Errorf("get %q 超时", key))
	return ""
}

func (d *demoClient) put(kvs []*raft.KVServer, key, value string) {
	d.seq++
	d.putAppend(kvs, raft.OpPut, key, value)
}

func (d *demoClient) append(kvs []*raft.KVServer, key, value string) {
	d.seq++
	d.putAppend(kvs, raft.OpAppend, key, value)
}

func (d *demoClient) putAppend(kvs []*raft.KVServer, kind, key, value string) {
	args := &raft.PutAppendArgs{Kind: kind, Key: key, Value: value, Client: d.id, Seq: d.seq}
	for attempt := 0; attempt < 400; attempt++ {
		for _, kv := range kvs {
			if kv == nil {
				continue
			}
			reply := raft.PutAppendReply{}
			if err := kv.PutAppend(args, &reply); err != nil {
				continue
			}
			if reply.Err == "" {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	fatal(fmt.Errorf("%s %q 超时", kind, key))
}

const leaderTimeout = 10 * time.Second

// waitLeader 取任期最高的 leader。被隔离的旧 leader 仍自认为是 leader，
// 但它的任期一定低于新选出来的节点，所以按任期取最大值即可。
func waitLeader(rafts []*raft.Raft) int {
	deadline := time.Now().Add(leaderTimeout)
	for time.Now().Before(deadline) {
		best, bestTerm := -1, -1
		for i, rf := range rafts {
			if rf == nil {
				continue
			}
			term, isLeader := rf.GetState()
			if isLeader && term > bestTerm {
				best, bestTerm = i, term
			}
		}
		if best >= 0 {
			return best
		}
		time.Sleep(20 * time.Millisecond)
	}
	fatal(fmt.Errorf("等待 leader 超时"))
	return -1
}

func termOf(rf *raft.Raft) int {
	term, _ := rf.GetState()
	return term
}

func parsePeers(spec string) ([]string, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, fmt.Errorf("必须用 -peers 指定集群地址")
	}
	addrs := map[int]string{}
	maxID := -1
	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("无法解析节点地址 %q，格式应为 id=host:port", item)
		}
		id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || id < 0 {
			return nil, fmt.Errorf("非法节点编号 %q", parts[0])
		}
		addrs[id] = strings.TrimSpace(parts[1])
		if id > maxID {
			maxID = id
		}
	}
	out := make([]string, maxID+1)
	for id, addr := range addrs {
		out[id] = addr
	}
	for id := 0; id <= maxID; id++ {
		if out[id] == "" {
			return nil, fmt.Errorf("节点编号必须从 0 开始连续，缺少 %d", id)
		}
	}
	return out, nil
}

func splitServers(spec string) []string {
	var out []string
	for _, item := range strings.Split(spec, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "错误:", err)
	os.Exit(1)
}
