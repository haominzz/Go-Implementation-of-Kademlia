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
 	"time"
 	"sort"
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
    //responseChan Channel
}

type flagedContact struct{
	contactInfo Contact
	flag int // 1: active -1: inactive 0:default
	prefixLen int
}

type respondInfo struct{
	findValueRes FindValueResult
	findNodeRes	FindNodeResult
	SenderID ID
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
	selfId := k.NodeID
	index := selfId.Xor(nodeId).PrefixLen()
	ele, err := k.RoutingTable[index].findContact(nodeId)
	if err != nil{
		return nil
	}
	
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
	ping.Sender = k.SelfContact
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

	var outString string
	for _, node := range res.Nodes{
		outString += node.NodeID.AsString()
		outString += "\n"
	}
	return "Successfully find: \n" + outString
}

func (k *Kademlia) FindNodeIter(contact *Contact, searchKey ID, channel chan respondInfo) string{
	//used for DoIterativeFindNode
	connectAddress := contact.Host.String() + ":" + strconv.Itoa(int(contact.Port))
	client, err := rpc.DialHTTP("tcp", connectAddress)
	if err != nil{
		return "internalFindNode failed for Dail Fails"
	}

	req := new(FindNodeRequest)
	req.Sender = *contact
	req.MsgID = NewRandomID()
	req.NodeID = searchKey
	var res FindNodeResult
	var res_v FindValueResult
	err = client.Call("KademliaCore.FindNode", req, &res)
	if err != nil{
		return "Fail to Find Node"
	}else{
		//for iterative find node 
		channel <- (respondInfo{res_v, res, contact.NodeID})
	}

	//update channel
	selfId := k.SelfContact.NodeID
	if selfId != contact.NodeID{
		index := selfId.Xor(contact.NodeID).PrefixLen()
		k.RoutingTable[index].update(*contact)
	}
	return "Successfully FindNodeIter!"
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

	var outString string
	if len(res.Value) > 0{
		outString += "Successfully find value: "
		s := string(res.Value[:])
		outString += s
	}else{
		outString += "Faied to find value, find nodes instead:\n"
		for _, node := range res.Nodes{
			outString += node.NodeID.AsString()
			outString += "\n"
		}
	}
	return outString
	
}

func (k *Kademlia) DoFindValueIter(contact *Contact, searchKey ID, channel chan respondInfo) string{
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
	var res_n FindNodeResult
	err = client.Call("KademliaCore.FindValue",req, &res)
	if err!= nil{
		return "Fail to Fina Value"
	}else{
		//for iterative find node 
		channel <- (respondInfo{res, res_n, contact.NodeID})
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
	
	responseChan := make(chan respondInfo, 100) 
	/* ---shourlist : used to track all visited nodes ---*/
	/* --- by map nodeID to index of corresponding node in cloestlist ---*/
	shortlist := make(map[ID]int)

	/*--- cloestlist : used to keep all the nodes in response ---*/
	/*--- keep the order from nearest to farest ---*/
	var cloestlist cloestNodes
	target := id

	/*--- track the prefixLen of the cloest node found ---*/
	/*--- the initial value is the prefixLen between target and current node ---*/
	// cloestNodePrefixLen_Old := 0
	// cloestNodePrefixLen_New := -1


	/*--- add the three initial nodes ---*/ 
	go k.FindNodeIter(&k.SelfContact, id, responseChan)

	/*--- in the respond of previous FindNodeIter, there is not SelfContact ---*/
	/*--- add SelfContact to shortlist and cloestlist ---*/
	prefixLen_init := target.Xor(k.NodeID).PrefixLen()
	index_init := len(cloestlist)
	cloestlist = append(cloestlist, flagedContact{k.SelfContact, 0, prefixLen_init})
	shortlist[k.NodeID] = index_init

	// wait for 300 ms
	time.Sleep(300 * time.Millisecond)

	/*--- check the two termination conditions ---*/
	/*--- (1) no Contacts returned are closer than currently exist in the shortlist, ---/
	/---- (2) there are k active contacts in the shortlist.	---/

	///////////////////////////////--- check termination ---/////////////////////////////
	/---- (1) satisfied then check (2), if (2) also satisfied, return ---/
	/---- (1) satisfied (2) doesn't, find K in shortlist which is unvisited, send rpcs ---/
	/---- (2) satisfied, return ---*/
	fmt.Println(strconv.FormatBool(cloestlist.hasKActive()))

	for !(cloestlist.hasKActive()){
		fmt.Println("======== cloestlist.hasKActive = False ========")
		for len(responseChan) > 0{
				fmt.Println("======== Read From Channel ========")
				respack := <- responseChan
				fmt.Println("======== Content in Response =======")

				/*--- get information from respack: ---/
				/---- respondInfo{findValueRes FindValueResult, findNodeRes FindNodeResult, SenderID ID} ---*/
				sender := respack.SenderID
				contactList := respack.findNodeRes.Nodes
				fmt.Println("sender: " + sender.AsString())
				fmt.Println("length of contactlist = " + strconv.Itoa(len(contactList)))
				if len(contactList) > 0{
					fmt.Println("id of node" + contactList[0].NodeID.AsString())	
				}
				

				/*--- Append all unvisied nodes in responses to closetlist ---*/
				for _, node := range contactList{
					/*--- to check whether node is in map, that means whether node has been visited ---*/
					/*--- if not in map, store it to cloestlist, and store the index of the node in map ---*/
					if _, ok := shortlist[node.NodeID]; !ok{
						fmt.Println("node not in map")
						//count the prefix length from curnode to target
						prefixLen := target.Xor(node.NodeID).PrefixLen()
						index := len(cloestlist)
						cloestlist = append(cloestlist, flagedContact{node, 0, prefixLen})
						shortlist[node.NodeID] = index
						fmt.Println("====== After adding node, len(cloestlist) = " + strconv.Itoa(len(cloestlist)))
					}	
				}
				fmt.Println("======== Append All Unvisited Nodes To cloestlist ========")


				/*--- change the flag of sender to active 1 ---*/
				fmt.Println("===== Change the flag to active ========")
				fmt.Println("senderID: " + sender.AsString())
				index := shortlist[sender]
				fmt.Println("index of sender: " + strconv.Itoa(index))
				fmt.Println("len(cloestlist) = " + strconv.Itoa(len(cloestlist)))
				cloestlist[index].flag = 1


				
		}

		/*--- Remove all the inactive nodes in cloestlist ---*/
		var templist cloestNodes
		cloestLength := len(cloestlist)
		for i := 0; i < cloestLength; i++{
			if cloestlist[i].flag != -1{
				//fmt.Println("======= remove node:" + cloestlist[i].contactInfo.NodeID.AsString())
				//cloestlist = append(cloestlist[:i],cloestlist[i+1:]...)
				templist = append(templist, cloestlist[i])
			}
		}
		cloestlist = templist
		fmt.Println("======== Removed All Inactive Nodes In cloestlist ========")
		/*--- Remove all the inactive nodes in cloestlist ---*/   //bug
				for j,node:= range cloestlist{
					fmt.Println("Node: ",j,node.contactInfo.NodeID.AsString(),"flag:",node.flag)
				}
				fmt.Println("==================================End=================================================")

		/*--- get the old prefixLen of the cloest node ---*/
		// cloestNodePrefixLen_Old = cloestlist[0].prefixLen
		if len(cloestlist)!= 0{
			sort.Sort(cloestNodes(cloestlist))
			/*--- get the new prefixLen of the cloest node ---*/
			// cloestNodePrefixLen_New = cloestlist[0].prefixLen
			/*--- asigned the correct index of nodes to map after sort ---*/
			for i, node := range cloestlist{
				shortlist[node.contactInfo.NodeID] = i
			}
			fmt.Println("======== After Sort ========")
		}	

		

							
				
		/*--- pick three contact with flag 0 in shortlist, send rpcs ---*/
		for i, _ := range cloestlist{
			counter := 0
			if counter == alpha{
				break
			}else{
				if cloestlist[i].flag == 0{
					cloestlist[i].flag = -1
					counter += 1
					fmt.Println("======== Send RPC ========")

					go k.FindNodeIter(&cloestlist[i].contactInfo, id, responseChan)

				}
			}
			
		}

		// wait for 300 ms
		time.Sleep(300 * time.Millisecond)
	
	}

	var outString string
	total := len(cloestlist)
	fmt.Println(total)
	if K < total{
		total = K
	}
	for i := 0; i < total; i++{
		fmt.Println(total)
		outString += cloestlist[i].contactInfo.NodeID.AsString()
		outString += "\n"
	}
	return "Successfully Find Contacts: \n" + outString
}


/*--- DoIterativeFindNode which return k contact, called in DoIterativeFindValue ---*/
func (k *Kademlia) DoIterativeFindNodeForValue(id ID) []flagedContact {

	responseChan := make(chan respondInfo, 100) 
	/* ---shourlist : used to track all visited nodes ---*/
	/* --- by map nodeID to index of corresponding node in cloestlist ---*/
	shortlist := make(map[ID]int)

	/*--- cloestlist : used to keep all the nodes in response ---*/
	/*--- keep the order from nearest to farest ---*/
	var cloestlist cloestNodes
	target := id

	/*--- track the prefixLen of the cloest node found ---*/
	/*--- the initial value is the prefixLen between target and current node ---*/
	// cloestNodePrefixLen_Old := 0
	// cloestNodePrefixLen_New := -1


	/*--- add the three initial nodes ---*/ 
	go k.FindNodeIter(&k.SelfContact, id, responseChan)

	/*--- in the respond of previous FindNodeIter, there is not SelfContact ---*/
	/*--- add SelfContact to shortlist and cloestlist ---*/
	prefixLen_init := target.Xor(k.NodeID).PrefixLen()
	index_init := len(cloestlist)
	cloestlist = append(cloestlist, flagedContact{k.SelfContact, 0, prefixLen_init})
	shortlist[k.NodeID] = index_init

	// wait for 300 ms
	time.Sleep(300 * time.Millisecond)

	/*--- check the two termination conditions ---*/
	/*--- (1) no Contacts returned are closer than currently exist in the shortlist, ---/
	/---- (2) there are k active contacts in the shortlist.	---/

	///////////////////////////////--- check termination ---/////////////////////////////
	/---- (1) satisfied then check (2), if (2) also satisfied, return ---/
	/---- (1) satisfied (2) doesn't, find K in shortlist which is unvisited, send rpcs ---/
	/---- (2) satisfied, return ---*/
	fmt.Println(strconv.FormatBool(cloestlist.hasKActive()))

	for !(cloestlist.hasKActive()){
		fmt.Println("======== cloestlist.hasKActive = False ========")
		for len(responseChan) > 0{
				fmt.Println("======== Read From Channel ========")
				respack := <- responseChan
				fmt.Println("======== Content in Response =======")

				/*--- get information from respack: ---/
				/---- respondInfo{findValueRes FindValueResult, findNodeRes FindNodeResult, SenderID ID} ---*/
				sender := respack.SenderID
				contactList := respack.findNodeRes.Nodes
				fmt.Println("sender: " + sender.AsString())
				fmt.Println("length of contactlist = " + strconv.Itoa(len(contactList)))
				if len(contactList) > 0{
					fmt.Println("id of node" + contactList[0].NodeID.AsString())	
				}
				

				/*--- Append all unvisied nodes in responses to closetlist ---*/
				for _, node := range contactList{
					/*--- to check whether node is in map, that means whether node has been visited ---*/
					/*--- if not in map, store it to cloestlist, and store the index of the node in map ---*/
					if _, ok := shortlist[node.NodeID]; !ok{
						fmt.Println("node not in map")
						//count the prefix length from curnode to target
						prefixLen := target.Xor(node.NodeID).PrefixLen()
						index := len(cloestlist)
						cloestlist = append(cloestlist, flagedContact{node, 0, prefixLen})
						shortlist[node.NodeID] = index
						fmt.Println("====== After adding node, len(cloestlist) = " + strconv.Itoa(len(cloestlist)))
					}	
				}
				fmt.Println("======== Append All Unvisited Nodes To cloestlist ========")


				/*--- change the flag of sender to active 1 ---*/
				fmt.Println("===== Change the flag to active ========")
				fmt.Println("senderID: " + sender.AsString())
				index := shortlist[sender]
				fmt.Println("index of sender: " + strconv.Itoa(index))
				fmt.Println("len(cloestlist) = " + strconv.Itoa(len(cloestlist)))
				cloestlist[index].flag = 1


				
		}

		/*--- Remove all the inactive nodes in cloestlist ---*/
		var templist cloestNodes
		cloestLength := len(cloestlist)
		for i := 0; i < cloestLength; i++{
			if cloestlist[i].flag != -1{
				//fmt.Println("======= remove node:" + cloestlist[i].contactInfo.NodeID.AsString())
				//cloestlist = append(cloestlist[:i],cloestlist[i+1:]...)
				templist = append(templist, cloestlist[i])
			}
		}
		cloestlist = templist
		fmt.Println("======== Removed All Inactive Nodes In cloestlist ========")
		/*--- Remove all the inactive nodes in cloestlist ---*/   //bug
				for j,node:= range cloestlist{
					fmt.Println("Node: ",j,node.contactInfo.NodeID.AsString(),"flag:",node.flag)
				}
				fmt.Println("==================================End=================================================")

		/*--- get the old prefixLen of the cloest node ---*/
		// cloestNodePrefixLen_Old = cloestlist[0].prefixLen
		if len(cloestlist)!= 0{
			sort.Sort(cloestNodes(cloestlist))
			/*--- get the new prefixLen of the cloest node ---*/
			// cloestNodePrefixLen_New = cloestlist[0].prefixLen
			/*--- asigned the correct index of nodes to map after sort ---*/
			for i, node := range cloestlist{
				shortlist[node.contactInfo.NodeID] = i
			}
			fmt.Println("======== After Sort ========")
		}	

		

							
				
		/*--- pick three contact with flag 0 in shortlist, send rpcs ---*/
		for i, _ := range cloestlist{
			counter := 0
			if counter == alpha{
				break
			}else{
				if cloestlist[i].flag == 0{
					cloestlist[i].flag = -1
					counter += 1
					fmt.Println("======== Send RPC ========")

					go k.FindNodeIter(&cloestlist[i].contactInfo, id, responseChan)

				}
			}
			
		}

		// wait for 300 ms
		time.Sleep(300 * time.Millisecond)
	
	}

	var outString string
	total := len(cloestlist)
	fmt.Println(total)
	if K < total{
		total = K
	}
	for i := 0; i < total; i++{
		fmt.Println(total)
		outString += cloestlist[i].contactInfo.NodeID.AsString()
		outString += "\n"
	}
	
	return cloestlist[:total]
}



type  cloestNodes []flagedContact

func (n cloestNodes) Len() int{
	return len(n)
}

func (n cloestNodes) Less(i, j int) bool{
	return n[i].prefixLen < n[j].prefixLen
} 

func (n cloestNodes) Swap(i, j int){
	n[i], n[j] = n[j], n[i]
}

/*--- used to check whether cloestlist contains K active cloestest Nodes in the first K contact ---*/
func (n cloestNodes) hasKActive() bool{
	counterK := 0
	//counterUnvisted := 0
	ret := false
	if len(n) > K{
	/*--- len(n) > K, the previous K nodes are all active, return true ---*/
		for i:= 0; i < K; i++{
			/*--- count the number of active in previous 20 nodes ---*/
			if n[i].flag == 1{
				counterK += 1
			}
		}
		ret = (counterK == K)
	}else{
	/*--- len(n) < K, all nodes in cloestlist are active, return true ---*/
		for _, node := range n{
			if node.flag == 1{
				counterK += 1
			}
		}
		if len(n) != 0 && counterK == len(n){
			ret = true
		}
	}
	
	return ret
}


func (k *Kademlia) DoIterativeStore(key ID, value []byte) string {
	// first calls iterativeFindNode() and receives a set of k triples
	contactList := k.DoIterativeFindNodeForValue(key)
	retString := ""

	// for i := 0; i < len(contactList); i++{
	// 	retString += contactList[i].contactInfo.NodeID.AsString()
	// 	retString += "\n"
	// }

	//STORE rpc is sent to each of these Contacts
	for i, _ := range contactList{
		go k.DoStore(&contactList[i].contactInfo, key, value)
		retString = contactList[i].contactInfo.NodeID.AsString()
	}
	//returns a string containing the ID of the node that received the final STORE operation
	return "The last one received the rps is: \n" +  retString
}

func (k *Kademlia) DoIterativeFindValue(key ID) string {

	responseChan := make(chan respondInfo, 100) 
	/* ---shourlist : used to track all visited nodes ---*/
	/* --- by map nodeID to index of corresponding node in cloestlist ---*/
	shortlist := make(map[ID]int)

	/*--- cloestlist : used to keep all the nodes in response ---*/
	/*--- keep the order from nearest to farest ---*/
	var cloestlist cloestNodes
	target := key

	/*--- track the prefixLen of the cloest node found ---*/
	/*--- the initial value is the prefixLen between target and current node ---*/
	// cloestNodePrefixLen_Old := 0
	// cloestNodePrefixLen_New := -1


	/*--- add the three initial nodes ---*/ 
	go k.DoFindValueIter(&k.SelfContact, key, responseChan)

	prefixLen_init := target.Xor(k.NodeID).PrefixLen()
	index_init := len(cloestlist)
	cloestlist = append(cloestlist, flagedContact{k.SelfContact, 0, prefixLen_init})
	shortlist[k.NodeID] = index_init

	// wait for 300 ms
	time.Sleep(300 * time.Millisecond)

	// check the three termination conditions
	//(1) if at any point the value is returned instead of a set of Contacts (i.e. the value is found)
	//(2) no Contacts returned are closer than currently exist in the shortlist, 
	//(3) there are k active contacts in the shortlist.
	for !(cloestlist.hasKActive()){
		for len(responseChan) > 0{
			respack := <- responseChan

			/*--- get information from respack: ---/
			/---- respondInfo{findValueRes FindValueResult, findNodeRes FindNodeResult, SenderID ID} ---*/
			sender := respack.SenderID
			contactList := respack.findValueRes.Nodes
			value := respack.findValueRes.Value

			/*--- check the (1) contdition of termination ---*/
			if len(value) > 0{
				s := string(value[:])
				return "Successfully find value: " + s
			}

			/*--- Append all unvisied nodes in responses to closetlist ---*/
			for _, node := range contactList{
				/*--- to check whether node is in map, that means whether node has been visited ---*/
				/*--- if not in map, store it to cloestlist, and store the index of the node in map ---*/
				if _, ok := shortlist[node.NodeID]; !ok{
					fmt.Println("node not in map")
					//count the prefix length from curnode to target
					prefixLen := target.Xor(node.NodeID).PrefixLen()
					index := len(cloestlist)
					cloestlist = append(cloestlist, flagedContact{node, 0, prefixLen})
					shortlist[node.NodeID] = index
					fmt.Println("====== After adding node, len(cloestlist) = " + strconv.Itoa(len(cloestlist)))
				}	
			}
			fmt.Println("======== Append All Unvisited Nodes To cloestlist ========")


			/*--- change the flag of sender to active 1 ---*/
			fmt.Println("===== Change the flag to active ========")
			fmt.Println("senderID: " + sender.AsString())
			index := shortlist[sender]
			fmt.Println("index of sender: " + strconv.Itoa(index))
			fmt.Println("len(cloestlist) = " + strconv.Itoa(len(cloestlist)))
			cloestlist[index].flag = 1
			
			
				
		}

		/*--- Remove all the inactive nodes in cloestlist ---*/
		var templist cloestNodes
		cloestLength := len(cloestlist)
		for i := 0; i < cloestLength; i++{
			if cloestlist[i].flag != -1{
				//fmt.Println("======= remove node:" + cloestlist[i].contactInfo.NodeID.AsString())
				//cloestlist = append(cloestlist[:i],cloestlist[i+1:]...)
				templist = append(templist, cloestlist[i])
			}
		}
		cloestlist = templist
		fmt.Println("======== Removed All Inactive Nodes In cloestlist ========")

		/*--- get the old prefixLen of the cloest node ---*/
		// cloestNodePrefixLen_Old = cloestlist[0].prefixLen
		if len(cloestlist)!= 0{
			sort.Sort(cloestNodes(cloestlist))
			/*--- get the new prefixLen of the cloest node ---*/
			// cloestNodePrefixLen_New = cloestlist[0].prefixLen
			/*--- asigned the correct index of nodes to map after sort ---*/
			for i, node := range cloestlist{
				shortlist[node.contactInfo.NodeID] = i
			}
			fmt.Println("======== After Sort ========")
		}

		/*--- pick three contact with flag 0 in shortlist, send rpcs ---*/
		for i, _ := range cloestlist{
			counter := 0
			if counter == alpha{
				break
			}else{
				if cloestlist[i].flag == 0{
					cloestlist[i].flag = -1
					counter += 1
					fmt.Println("======== Send RPC ========")

					go k.DoFindValueIter(&cloestlist[i].contactInfo, key, responseChan)

				}
			}
			
		}

		// wait for 300 ms
		time.Sleep(300 * time.Millisecond)
	
	}

	var outString string
	total := len(cloestlist)
	fmt.Println(total)
	if K < total{
		total = K
	}
	for i := 0; i < total; i++{
		fmt.Println(total)
		outString += cloestlist[i].contactInfo.NodeID.AsString()
		outString += "\n"
	}
	return "Faided to find value, find contacts instead \n" + outString

}
