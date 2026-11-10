package raft

import (
	"math/rand"
	"net"
	"net/rpc"
	"time"
)

// Clerk 是键值服务的客户端。它记住上一次成功的节点，失败就依次换下一个。
// 重试时使用同一个 Seq，服务端据此去重，所以“至少一次”的 RPC 不会重复执行。
// Clerk 不是并发安全的：一个 goroutine 用一个 Clerk。
type Clerk struct {
	servers []string
	leader  int
	client  int64
	seq     int
}

func NewClerk(servers []string) *Clerk {
	return &Clerk{servers: servers, client: rand.Int63(), seq: 0}
}

func (c *Clerk) Get(key string) string {
	c.seq++
	args := GetArgs{Key: key, Client: c.client, Seq: c.seq}
	for {
		for i := 0; i < len(c.servers); i++ {
			server := (c.leader + i) % len(c.servers)
			reply := GetReply{}
			if c.call(server, "KV.Get", &args, &reply) && reply.Err == "" {
				c.leader = server
				return reply.Value
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (c *Clerk) Put(key, value string) {
	c.putAppend(OpPut, key, value)
}

func (c *Clerk) Append(key, value string) {
	c.putAppend(OpAppend, key, value)
}

func (c *Clerk) putAppend(kind, key, value string) {
	c.seq++
	args := PutAppendArgs{Kind: kind, Key: key, Value: value, Client: c.client, Seq: c.seq}
	for {
		for i := 0; i < len(c.servers); i++ {
			server := (c.leader + i) % len(c.servers)
			reply := PutAppendReply{}
			if c.call(server, "KV.PutAppend", &args, &reply) && reply.Err == "" {
				c.leader = server
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (c *Clerk) call(server int, method string, args, reply interface{}) bool {
	conn, err := net.DialTimeout("tcp", c.servers[server], 300*time.Millisecond)
	if err != nil {
		return false
	}
	client := rpc.NewClient(conn)
	defer client.Close()

	call := client.Go(method, args, reply, make(chan *rpc.Call, 1))
	select {
	case <-call.Done:
		return call.Error == nil
	case <-time.After(time.Second):
		return false
	}
}
