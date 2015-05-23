package kademlia

// Contains definitions mirroring the Kademlia spec. You will need to stick
// strictly to these to be compatible with the reference implementation and
// other groups' code.

import (
	"net"
	"fmt"
	//"strconv"
)

type KademliaCore struct {
	kademlia *Kademlia
}

// Host identification.
type Contact struct {
	NodeID ID
	Host   net.IP
	Port   uint16
}

///////////////////////////////////////////////////////////////////////////////
// PING
///////////////////////////////////////////////////////////////////////////////
type PingMessage struct {
	Sender Contact
	MsgID  ID
}

type PongMessage struct {
	MsgID  ID
	Sender Contact
}

func (kc *KademliaCore) Ping(ping PingMessage, pong *PongMessage) error {
	// TODO: Finish implementation
	//handle the condition to ping myself
	kc.kademlia.lock.Lock()
	selfId := kc.kademlia.SelfContact.NodeID
	pong.MsgID = CopyID(ping.MsgID)
	// Specify the sender
	pong.Sender = kc.kademlia.SelfContact
	if ping.Sender.NodeID != selfId{
		// Update contact, etc
		selfId := kc.kademlia.SelfContact.NodeID
		index := selfId.Xor(ping.Sender.NodeID).PrefixLen()
		kc.kademlia.RoutingTable[index].update(ping.Sender)//should use channel
	}
   
	kc.kademlia.lock.Unlock()
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// STORE
///////////////////////////////////////////////////////////////////////////////
type StoreRequest struct {
	Sender Contact
	MsgID  ID
	Key    ID
	Value  []byte
}

type StoreResult struct {
	MsgID ID
	Err   error
}

func (kc *KademliaCore) Store(req StoreRequest, res *StoreResult) error {
	// TODO: Implement.
	kc.kademlia.lock.Lock()
	fmt.Println("in rpcs Store")
	res.MsgID = CopyID(req.MsgID)
	kc.kademlia.ValueTable[req.Key] = req.Value

	// update channel
	selfId := kc.kademlia.SelfContact.NodeID
	if req.Sender.NodeID != selfId{
		selfId := kc.kademlia.SelfContact.NodeID
		index := selfId.Xor(req.Sender.NodeID).PrefixLen()
		kc.kademlia.RoutingTable[index].update(req.Sender)
		res.Err = nil
		//??what is Err
	}
	kc.kademlia.lock.Unlock()
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// FIND_NODE
///////////////////////////////////////////////////////////////////////////////
type FindNodeRequest struct {
	Sender Contact
	MsgID  ID
	NodeID ID 
}

type FindNodeResult struct {
	MsgID ID
	Nodes []Contact
	Err   error
}

func (kc *KademliaCore) FindNode(req FindNodeRequest, res *FindNodeResult) error {
	// TODO: Implement.
	fmt.Println("in rpcs FindNode")
	kc.kademlia.lock.Lock()
	requestId := req.NodeID
	selfId := kc.kademlia.SelfContact.NodeID
	var index int 
	if requestId == selfId{
		index = B - 1 
	}else{
		index = selfId.Xor(requestId).PrefixLen()
	}

	res.MsgID = CopyID(req.MsgID)
	res.Nodes = kc.kademlia.RoutingTable[index].copyLine()
	//if the current KBueckt does not contain K contacts
	if len(res.Nodes) < K{
		// find nodes front
		
		for i := index-1; i >= 0; i--{
			for e := kc.kademlia.RoutingTable[i].Front(); e != nil; e = e.Next(){
				if len(res.Nodes) == K{
					break
				}else{
					c := e.Value.(Contact)
					res.Nodes = append(res.Nodes,c)
				}
			}
		}
		// add all nodes in previous buckets, len < K, add behand
		if len(res.Nodes) < K{
			for i := index+1; i < len(kc.kademlia.RoutingTable)-1; i++{
				for e := kc.kademlia.RoutingTable[i].Front(); e != nil; e = e.Next(){
					if len(res.Nodes) == K{
						break
					}else{
						c := e.Value.(Contact)
						res.Nodes = append(res.Nodes,c)
					}
				}
			}
		}
	}
	res.Err = nil

	//update channel

	if req.Sender.NodeID != selfId{
		kc.kademlia.RoutingTable[index].update(req.Sender)
	}
	kc.kademlia.lock.Unlock()
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// FIND_VALUE
///////////////////////////////////////////////////////////////////////////////
type FindValueRequest struct {
	Sender Contact
	MsgID  ID
	Key    ID
}

// If Value is nil, it should be ignored, and Nodes means the same as in a
// FindNodeResult.
type FindValueResult struct {
	MsgID ID
	Value []byte
	Nodes []Contact
	Err   error
}

func (kc *KademliaCore) FindValue(req FindValueRequest, res *FindValueResult) error {
	// TODO: Implement.
	fmt.Println("in rpcs FindValue")
	kc.kademlia.lock.Lock()
	res.MsgID = CopyID(req.MsgID)
	value, ok := kc.kademlia.ValueTable[req.Key]
	selfId := kc.kademlia.SelfContact.NodeID
	if ok{
		res.Value = value
		//update channel
		if req.Sender.NodeID != selfId{
			index := selfId.Xor(req.Sender.NodeID).PrefixLen()
			kc.kademlia.RoutingTable[index].update(req.Sender)
		}
	}else{
		var index int
		if req.Key == selfId{
			index = B - 1 
		}else{
			index = selfId.Xor(req.Key).PrefixLen()
		}
		res.Nodes = kc.kademlia.RoutingTable[index].copyLine()
		/*--- if len(res.Nodes) < K ---*/
		if len(res.Nodes) < K{
		// find nodes front
		
			for i := index-1; i >= 0; i--{
				for e := kc.kademlia.RoutingTable[i].Front(); e != nil; e = e.Next(){
					if len(res.Nodes) == K{
						break
					}else{
						c := e.Value.(Contact)
						res.Nodes = append(res.Nodes,c)
					}
				}
			}
			// add all nodes in previous buckets, len < K, add behand
			if len(res.Nodes) < K{
				for i := index+1; i < len(kc.kademlia.RoutingTable)-1; i++{
					for e := kc.kademlia.RoutingTable[i].Front(); e != nil; e = e.Next(){
						if len(res.Nodes) == K{
							break
						}else{
							c := e.Value.(Contact)
							res.Nodes = append(res.Nodes,c)
						}
					}
				}
			}
		}
		if req.Sender.NodeID != selfId{
			//update
			kc.kademlia.RoutingTable[index].update(req.Sender)
		}
	
	}
	kc.kademlia.lock.Unlock()
	return nil
}
