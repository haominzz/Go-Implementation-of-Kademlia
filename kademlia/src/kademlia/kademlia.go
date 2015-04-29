package kademlia

// Contains the core kademlia type. In addition to core state, this type serves
// as a receiver for the RPC methods, which is required by that package.

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
    "strconv"
 	"sync"
)

const (
	alpha = 3
	B     = 8 * IDBytes
	K     = 20
)

// Kademlia type. You can put whatever state you need in this.
type Kademlia struct {
	NodeID ID
    SelfContact Contact
    RoutingTable []*KBucket
    ValueTable map[ID] []byte
    lock sync.Mutex
}




func NewKademlia(laddr string) *Kademlia {
	// TODO: Initialize other state here as you add functionality.
	k := new(Kademlia)
	k.NodeID = NewRandomID()
	k.RoutingTable = make([]*KBucket, B)
	for ele, _ := range k.RoutingTable{
		k.RoutingTable[ele] = NewKBucket()
	}
	k.ValueTable = make(map[ID] []byte)
	k.lock = sync.Mutex{} 
	// Set up RPC server
	// NOTE: KademliaCore is just a wrapper around Kademlia. This type includes
	// the RPC functions.
	rpc.Register(&KademliaCore{k})
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		log.Fatal("Listen: ", err)
	}

	// Run RPC server forever.
	go http.Serve(l, nil)

    // Add self contact
    hostname, port, _ := net.SplitHostPort(l.Addr().String())
    port_int, _ := strconv.Atoi(port)
    ipAddrStrings, err := net.LookupHost(hostname)
    var host net.IP
    for i := 0; i < len(ipAddrStrings); i++ {
        host = net.ParseIP(ipAddrStrings[i])
        if host.To4() != nil {
            break
        }
    }
    k.SelfContact = Contact{k.NodeID, host, uint16(port_int)}
	return k
}


type NotFoundError struct {
	id  ID
	msg string
}


func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%x %s", e.id, e.msg)
}

func (k *Kademlia) findContactFromKTable(nodeId ID) (*Contact){
	selfId := k.SelfContact.NodeID
	index := selfId.Xor(nodeId).PrefixLen()
	ele, _ := k.RoutingTable[index].findContact(nodeId)
	contact := ele.Value.(Contact)
	return &contact
}

func (k *Kademlia) FindContact(nodeId ID) (*Contact, error) {
	// TODO: Search through contacts, find specified ID
	// Find contact with provided ID
    if nodeId == k.SelfContact.NodeID {
        return &k.SelfContact, nil
    }
    cont := k.findContactFromKTable(nodeId)
    if cont != nil{
    	return cont, nil
    }
	return nil, &NotFoundError{nodeId, "Not found"}
}

// This is the function to perform the RPC
func (k *Kademlia) DoPing(host net.IP, port uint16) string {
	// TODO: Implement
	// If all goes well, return "OK: <output>", otherwise print "ERR: <messsage>"
	pingAddress := host.String() + ":" + strconv.Itoa(int(port))
	client, err := rpc.DialHTTP("tcp", pingAddress)
	if err != nil{
		log.Fatal("DialHTTP: ", err)
	}
	ping := new(PingMessage)
	ping.MsgID = NewRandomID()
	var pong PongMessage
	err = client.Call("KademliaCore.Ping", ping, &pong)
	if err != nil{
		//log.Fatal("Call: ", err)
		return "Failed to ping!"
	}
	
	//update,should use channel
	//??should I check msgID
	selfId := k.SelfContact.NodeID
	if selfId != pong.Sender.NodeID{
		index := selfId.Xor(pong.Sender.NodeID).PrefixLen()
		k.RoutingTable[index].update(pong.Sender)
	}
	
	return "Successfully ping " + host.String() + ":" + strconv.Itoa(int(port))

	
}

func (k *Kademlia) DoStore(contact *Contact, key ID, value []byte) string {
	// TODO: Implement
	// If all goes well, return "OK: <output>", otherwise print "ERR: <messsage>"
	fmt.Println("in kademlia DoStroe")
	connectAddress := contact.Host.String() + ":" + strconv.Itoa(int(contact.Port))
	client, err := rpc.DialHTTP("tcp", connectAddress)
	if err != nil{
		return "Store failed for Dial Fails"
	}

	req := new(StoreRequest)
	req.Sender = *contact
	req.MsgID = NewRandomID()
	req.Key = key
	req.Value = value
	var res StoreResult
	err = client.Call("KademliaCore.Store", req, &res)
	if err != nil{
		return "Failed to Store!"
	}
	//update channel

	selfId := k.SelfContact.NodeID
	if selfId != contact.NodeID{
		index := selfId.Xor(contact.NodeID).PrefixLen()
		k.RoutingTable[index].update(*contact)
	}
	
	return "Successfully store to " + contact.Host.String() + ":" + strconv.Itoa(int(contact.Port))
	
}

func (k *Kademlia) DoFindNode(contact *Contact, searchKey ID) string {
	// TODO: Implement
	// If all goes well, return "OK: <output>", otherwise print "ERR: <messsage>"
	connectAddress := contact.Host.String() + ":" + strconv.Itoa(int(contact.Port))
	client, err := rpc.DialHTTP("tcp", connectAddress)
	if err != nil{
		return "FindNode failed for Dail Fails"
	}

	req := new(FindNodeRequest)
	req.Sender = *contact
	req.MsgID = NewRandomID()
	req.NodeID = searchKey
	var res FindNodeResult
	err = client.Call("KademliaCore.FindNode", req, &res)
	if err != nil{
		return "Fail to Find Node"
	}

	//update channel
	selfId := k.SelfContact.NodeID
	if selfId != contact.NodeID{
		index := selfId.Xor(contact.NodeID).PrefixLen()
		k.RoutingTable[index].update(*contact)
	}

	return "Successfully find k nearest contacts of node " + searchKey.AsString()
}

func (k *Kademlia) DoFindValue(contact *Contact, searchKey ID) string {
	// TODO: Implement
	// If all goes well, return "OK: <output>", otherwise print "ERR: <messsage>"
	connectAddress := contact.Host.String() + ":" + strconv.Itoa(int(contact.Port))
	client, err := rpc.DialHTTP("tcp", connectAddress)
	if err != nil{
		return "FindValue failed for Dail Fails"
	}

	req := new(FindValueRequest)
	req.Sender = *contact
	req.MsgID = NewRandomID()
	req.Key = searchKey
	var res FindValueResult
	err = client.Call("KademliaCore.FindValue",req, &res)
	if err!= nil{
		return "Fail to Fina Value"
	}

	//update channel
	selfId := k.SelfContact.NodeID
	if selfId != contact.NodeID{
		index := selfId.Xor(contact.NodeID).PrefixLen()
		k.RoutingTable[index].update(*contact)
	}
	s := string(res.Value[:])
	return "Successfully find value: " + s
}

func (k *Kademlia) LocalFindValue(searchKey ID) string {
	// TODO: Implement
	// If all goes well, return "OK: <output>", otherwise print "ERR: <messsage>"
	value, ok := k.ValueTable[searchKey]
	if ok{
		s := string(value[:])
		return "Successfullu find value: " + s
	}
	return "Fails to find value locally"
}

func (k *Kademlia) DoIterativeFindNode(id ID) string {
	// For project 2!
	return "ERR: Not implemented"
}
func (k *Kademlia) DoIterativeStore(key ID, value []byte) string {
	// For project 2!
	return "ERR: Not implemented"
}
func (k *Kademlia) DoIterativeFindValue(key ID) string {
	// For project 2!
	return "ERR: Not implemented"
}
