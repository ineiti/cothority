package manage

import (
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

/*
Protocol used to close all connections, starting from the leaf-nodes.
*/

func init() {
	network.RegisterMessageType(PrepareCount{})
	network.RegisterMessageType(Count{})
	network.RegisterMessageType(NodeIsUp{})
	sda.ProtocolRegisterName("Count", NewCount)
}

type ProtocolCount struct {
	*sda.Node
	Replies          int
	Count            chan int
	Quit             chan bool
	Timeout          int
	timeoutLock      sync.Mutex
	NetworkDelay     int
	PrepareCountChan chan struct {
		*sda.TreeNode
		PrepareCount
	}
	CountChan    chan []CountMsg
	NodeIsUpChan chan struct {
		*sda.TreeNode
		NodeIsUp
	}
}

type PrepareCount struct {
	Timeout int
}
type NodeIsUp struct{}

type Count struct {
	Children int
}

type CountMsg struct {
	*sda.TreeNode
	Count
}

func NewCount(n *sda.Node) (sda.ProtocolInstance, error) {
	p := &ProtocolCount{
		Node:    n,
		Quit:    make(chan bool),
		Timeout: 1024,
		// This also includes the time to make a connection, eventually
		// re-try if the connection failed
		NetworkDelay: 100,
	}
	p.Count = make(chan int, 1)
	p.RegisterChannel(&p.CountChan)
	p.RegisterChannel(&p.PrepareCountChan)
	p.RegisterChannel(&p.NodeIsUpChan)
	return p, nil
}

// Myself nicely displays who we are
func (p *ProtocolCount) Myself() string {
	return fmt.Sprint(p.Entity().Addresses, p.Node.TokenID())
}

// Dispatch listens for all channels and waits for a timeout in case nothing
// happens for a certain duration
func (p *ProtocolCount) Dispatch() error {
	for {
		dbg.Lvl3(p.Myself(), "waiting for message during", p.Timeout)
		select {
		case pc := <-p.PrepareCountChan:
			dbg.Lvl3(p.Myself(), "received from", pc.TreeNode.Entity.Addresses,
				pc.Timeout)
			p.Timeout = pc.Timeout
			p.FuncPC()
		case c := <-p.CountChan:
			p.FuncC(c)
		case _ = <-p.NodeIsUpChan:
			if p.Parent() != nil {
				p.SendTo(p.Parent(), &NodeIsUp{})
			} else {
				p.Replies++
			}
		case <-time.After(time.Duration(p.Timeout) * time.Millisecond):
			dbg.Lvl3(p.Myself(), "timed out while waiting for", p.Timeout)
			if p.IsRoot() {
				dbg.Lvl2("Didn't get all children in time:", p.Replies)
				p.Count <- p.Replies
				p.Done()
				return nil
			}
		case _ = <-p.Quit:
			return nil
		}
	}
}

// FuncPC handles PrepareCount messages. These messages go down the tree and
// every node that receives one will reply with a 'NodeIsUp'-message
func (p *ProtocolCount) FuncPC() {
	if !p.IsRoot() {
		p.SendTo(p.Parent(), &NodeIsUp{})
	}
	if !p.IsLeaf() {
		for _, c := range p.Children() {
			dbg.Lvl3(p.Myself(), "sending to", c.Entity.Addresses, c.Id, p.Timeout)
			p.SendTo(c, &PrepareCount{Timeout: p.Timeout})
		}
	} else {
		p.FuncC(nil)
	}
}

// FuncC creates a Count-message that will be received by all parents and
// count the total number of children
func (p *ProtocolCount) FuncC(cc []CountMsg) {
	count := 1
	for _, c := range cc {
		count += c.Count.Children
	}
	if !p.IsRoot() {
		dbg.Lvl3(p.Myself(), "Sends to", p.Parent().Id, p.Parent().Entity.Addresses)
		p.SendTo(p.Parent(), &Count{count})
	} else {
		p.Count <- count
	}
	p.Quit <- true
	p.Done()
}

// Starts the protocol
func (p *ProtocolCount) Start() error {
	// Send an empty message
	dbg.Lvl3("Starting to count")
	go p.FuncPC()
	return nil
}
